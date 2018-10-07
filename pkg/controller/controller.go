// Copyright © 2018 Aviv Laufer <aviv.laufer@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package controller

import (
	"container/list"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/doitintl/kuberbs/pkg/api/types/v1"
	client_v1 "github.com/doitintl/kuberbs/pkg/clientset/v1"
	cfg "github.com/doitintl/kuberbs/pkg/config"
	"github.com/doitintl/kuberbs/pkg/deployment"
	"github.com/doitintl/kuberbs/pkg/metrics/datadog"
	"github.com/doitintl/kuberbs/pkg/metrics/metricsservice"
	"github.com/doitintl/kuberbs/pkg/metrics/stackdriver"
	"github.com/doitintl/kuberbs/pkg/utils"
	"github.com/sirupsen/logrus"
	apps_v1 "k8s.io/api/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const maxRetries = 5

var serverStartTime time.Time

// Event indicate the informerEvent
type Event struct {
	key          string
	eventType    string
	resourceType string
	old          *apps_v1.Deployment
	new          *apps_v1.Deployment
}

// Controller object
type Controller struct {
	logger        *logrus.Entry
	client        kubernetes.Interface
	clinetSet     *client_v1.V1Client
	queue         workqueue.RateLimitingInterface
	informer      cache.SharedIndexInformer
	config        *cfg.Config
	metricsSource string
	watchPeriod   int
}

// Start - starts the controller
func Start(config *cfg.Config) {
	var kubeClient kubernetes.Interface
	_, err := rest.InClusterConfig()
	if err != nil {
		logrus.Fatal(err)
	} else {
		kubeClient = utils.GetClient()
	}
	//start deployments watcher
	deploymentsInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				return kubeClient.AppsV1().Deployments(meta_v1.NamespaceAll).List(options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				return kubeClient.AppsV1().Deployments(meta_v1.NamespaceAll).Watch(options)
			},
		},
		&apps_v1.Deployment{},
		0, //Skip resync
		cache.Indexers{},
	)

	cd := newResourceController(kubeClient, deploymentsInformer, "deployment")
	cd.config = config
	deploymentsStopCh := make(chan struct{})
	defer close(deploymentsStopCh)

	go cd.Run(deploymentsStopCh)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func newResourceController(client kubernetes.Interface, informer cache.SharedIndexInformer, resourceType string) *Controller {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	var newEvent Event
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
		},
		UpdateFunc: func(old, new interface{}) {
			if utils.IsStrategySupported(string(new.(*apps_v1.Deployment).Spec.Strategy.Type)) {
				var err error
				newEvent.key, err = cache.MetaNamespaceKeyFunc(old)
				newEvent.eventType = "update"
				newEvent.resourceType = resourceType
				newEvent.old = old.(*apps_v1.Deployment)
				newEvent.new = new.(*apps_v1.Deployment)
				//logrus.WithFields(logrus.Fields{"pkg": "kuberbs-", "resourceType": resourceType, "newEvent.key": newEvent.key}).Debug("Processing update")
				if err == nil {
					queue.Add(newEvent)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
		},
	})
	var cfg *rest.Config
	cfg, err := rest.InClusterConfig()
	if err != nil {
		panic(err)
	}
	err = v1.AddToScheme(scheme.Scheme)
	if err != nil {
		panic(err)
	}
	clientSet, err := client_v1.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	return &Controller{
		logger:    logrus.WithFields(logrus.Fields{"service": "kuberbs", "pkg": "controler"}),
		client:    client,
		informer:  informer,
		queue:     queue,
		clinetSet: clientSet,
	}
}

// Run starts the kuberbs controller
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	c.logger.Info("Starting kuberbs controller")
	serverStartTime = time.Now().Local()
	c.loadDeployments()
	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		utilruntime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	c.logger.Info("kuberbs controller synced and ready")

	wait.Until(c.runWorker, time.Second, stopCh)
}

// HasSynced is required for the cache.Controller interface.
func (c *Controller) HasSynced() bool {
	return c.informer.HasSynced()
}

// LastSyncResourceVersion is required for the cache.Controller interface.
func (c *Controller) LastSyncResourceVersion() string {
	return c.informer.LastSyncResourceVersion()
}

func (c *Controller) runWorker() {
	for c.processNextItem() {
		// continue looping
	}
}

func (c *Controller) processNextItem() bool {
	newEvent, quit := c.queue.Get()

	if quit {
		return false
	}
	defer c.queue.Done(newEvent)
	err := c.processItem(newEvent.(Event))
	if err == nil {
		// No error, reset the ratelimit counters
		c.queue.Forget(newEvent)
	} else if c.queue.NumRequeues(newEvent) < maxRetries {
		c.logger.Errorf("Error processing %s (will retry): %v", newEvent.(Event).key, err)
		c.queue.AddRateLimited(newEvent)
	} else {
		// err != nil and too many retries
		c.logger.Errorf("Error processing %s (giving up): %v", newEvent.(Event).key, err)
		c.queue.Forget(newEvent)
		utilruntime.HandleError(err)
	}

	return true
}

func (c *Controller) processItem(newEvent Event) error {
	switch newEvent.eventType {
	case "update":
		var err error
		deploymentWatchList := deployment.GetDeploymentWatchList()
		status, metricName, threshold := c.getDataFromRbs(newEvent)
		if !status {
			return nil
		}
		isNewDp, dp := c.isNewDeployment(newEvent, deploymentWatchList)
		if isNewDp {
			newDeployment := deployment.NewDeploymentController(c.client, newEvent.old, newEvent.new, threshold)
			deploymentWatchList.PushBack(newDeployment)
			c.logger.WithFields(logrus.Fields{"namespace": newEvent.old.ObjectMeta.Namespace, "name": newEvent.old.ObjectMeta.Name}).Info("Got a deployment state change")
			return nil
		}
		if dp.Watching {
			newDeployment := deployment.NewDeploymentController(c.client, newEvent.old, newEvent.new, threshold)
			if newDeployment.ShouldWatch() {
				c.logger.Infof("Stopping watch for %s in %s", dp.Name, dp.NameSpace)
				dp.StopWatch(false)
				deployment.RemoveFromDeploymentWatchList(dp)
				deploymentWatchList.PushBack(newDeployment)
				return nil
			}
		}
		if dp.IsRollback {
			c.logger.Debugf("Rollback for %s is active", dp.Name)
			return nil
		}
		if !dp.DeploymentComplete(newEvent.new) {
			c.logger.Debugf("Deployment still in progress %s", dp.Name)
			return nil
		}
		if !dp.ShouldWatch() {
			// No change in containers version
			c.logger.Debugf("No change in %s deployment", dp.Name)
			deployment.RemoveFromDeploymentWatchList(dp)
			return nil
		}
		if !dp.IsObservedGenerationSame() {
			return nil
		}
		c.logger.Debugf("Saving deployment %s", dp.Name)
		err = dp.SaveCurrentDeploymentState()
		if err != nil {
			c.logger.Debugf("Will retry %s", err.Error())
			return nil
		}
		et := time.Now().Add(time.Duration(c.watchPeriod) * time.Minute)
		m := setWatcher(c.metricsSource, metricName, et, dp)
		m.Config = c.config
		go dp.StartWatch(*m)
		return nil

	case "create":
		return nil
	case "delete":
		return nil
	}
	return nil
}

func (c *Controller) isNewDeployment(newEvent Event, deploymentWatchList *list.List) (bool, *deployment.Deployment) {
	isNewDeployment := true
	var dp *deployment.Deployment
	for e := deploymentWatchList.Front(); e != nil; e = e.Next() {
		dp = e.Value.(*deployment.Deployment)
		if strings.EqualFold(dp.Name, newEvent.old.Name) && strings.EqualFold(dp.NameSpace, newEvent.old.Namespace) {
			isNewDeployment = false
			break
		}
	}
	c.logger.Debugf("%s is a new deployment %v", newEvent.old.Name, isNewDeployment)
	return isNewDeployment, dp
}

func (c *Controller) loadDeployments() {
	var metricName string
	rbs, err := c.clinetSet.Rbs(client_v1.RbsNameSpace).List(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	c.watchPeriod = rbs.Items[0].Spec.WatchPeriod
	c.metricsSource = strings.ToLower(rbs.Items[0].Spec.MetricsSource)
	for i, ns := range rbs.Items[0].Spec.Namespaces {
		for _, d := range rbs.Items[0].Spec.Namespaces[i].Deployments {
			dp, err := c.client.AppsV1().Deployments(ns.Name).Get(d.Deployment.Name, meta_v1.GetOptions{})
			if err != nil {
				c.logger.Error(err)
				continue
			}
			if meta_v1.HasAnnotation(dp.ObjectMeta, "kuberbs.starttime") {
				tstr := dp.ObjectMeta.Annotations["kuberbs.starttime"]
				if tstr != "" {
					t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", tstr)
					if err != nil {
						c.logger.Error(err)
						continue
					}
					t = t.Add(time.Duration(c.watchPeriod) * time.Minute)
					if time.Now().Before(t) {

						c.logger.WithFields(logrus.Fields{"namesapce": dp.Namespace, "name": dp.Name}).Info("Found a un finished watch task!")
						metricName = d.Deployment.Metric
						threshold := d.Deployment.Threshold
						old := c.createOldDeployment(dp)
						newDeployment := deployment.NewDeploymentController(c.client, old, dp, threshold)
						m := setWatcher(c.metricsSource, metricName, t, newDeployment)
						m.Config = c.config
						deploymentWatchList := deployment.GetDeploymentWatchList()
						deploymentWatchList.PushBack(newDeployment)
						go newDeployment.StartWatch(*m)
					}
				}
			}
		}
	}
}

func (c *Controller) createOldDeployment(current *apps_v1.Deployment) *apps_v1.Deployment {
	old := current
	for _, v := range current.Spec.Template.Spec.Containers {
		if meta_v1.HasAnnotation(current.ObjectMeta, "kuberbs.pod."+v.Name) {
			for i, item := range current.Spec.Template.Spec.Containers {
				if item.Name == v.Name {
					old.Spec.Template.Spec.Containers[i].Image = current.ObjectMeta.Annotations["kuberbs.pod."+v.Name]
				}
			}
		}
	}
	return old
}

func (c *Controller) getDataFromRbs(newEvent Event) (bool, string, int) {
	status := false
	var metricName string
	var threshold int

	rbs, err := c.clinetSet.Rbs(client_v1.RbsNameSpace).List(meta_v1.ListOptions{})
	if err != nil {
		panic(err)
	}
	for i, ns := range rbs.Items[0].Spec.Namespaces {
		if strings.EqualFold(ns.Name, newEvent.old.ObjectMeta.Namespace) {
			for _, dp := range rbs.Items[0].Spec.Namespaces[i].Deployments {
				if strings.EqualFold(dp.Deployment.Name, newEvent.old.ObjectMeta.Name) {
					status = true
					metricName = dp.Deployment.Metric
					threshold = dp.Deployment.Threshold
					continue
				}
			}
		}
	}
	return status, metricName, threshold
}

func setWatcher(metricsSource string, metricName string, et time.Time, newDeployment *deployment.Deployment) *metricsservice.MetricsSevrice {
	m := *metricsservice.NewMetricsSevrice(time.Now(), et, metricName)
	m.SetMetricsHandler(newDeployment.MetricsHandler)
	m.SetDoneHandler(newDeployment.WatchDoneHandler)
	switch metricsSource {
	case "stackdriver":
		m.SetCheckMetricsFunc(stackdriver.CheckMetrics)
		return &m
	case "datadog":
		m.SetCheckMetricsFunc(datadog.CheckMetrics)
		return &m
	default:
		return &m
	}
}
