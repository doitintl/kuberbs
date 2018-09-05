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

package metricsservice

import (
	"time"

	cfg "github.com/doitintl/kuberbs/pkg/config"
	"github.com/doitintl/kuberbs/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// dMetricsSevrice - Holds data about the MetricsSevrice watcher
type MetricsSevrice struct {
	StartAt          time.Time
	EndAt            time.Time
	logger           *logrus.Entry
	MetricName       string
	metricsHandler   metrics.ErrorRateHandler
	doneHandler      metrics.DoneHandler
	checkMetricsFunc metrics.CheckMetricsFunc
	doneChan         chan bool
	Config           *cfg.Config
}

// NewMetricsSevrice - create new MetricsSevrice struct
func NewMetricsSevrice(startat time.Time, endat time.Time, metricname string) *MetricsSevrice {
	return &MetricsSevrice{
		StartAt:    startat,
		EndAt:      endat,
		MetricName: metricname,
		logger:     logrus.WithFields(logrus.Fields{"service": "kuberbs", "pkg": "metricsevrice"}),
	}
}

// Start - start watching
func (ms *MetricsSevrice) Start() {
	ms.logger.Info("Starting MetricsSevrice Run")
	delta := time.Until(ms.EndAt)
	ms.doneChan = make(chan bool)
	timeChan := time.NewTimer(delta).C
	tickChan := time.NewTicker(time.Second * time.Duration(ms.Config.CheckMetricsInterval)).C
	for {
		select {
		case <-tickChan:
			logrus.Debug("Tick Called")
			rate, err := ms.checkMetricsFunc(ms.MetricName, ms.StartAt, ms.Config.APIKey, ms.Config.AppKey)
			if err != nil {
				ms.logger.Fatal(err)
			} else {
				ms.metricsHandler(rate)
			}
		case <-timeChan:
			ms.logger.Debug("Timer expired. We are done!")
			err := ms.doneHandler(true)
			if err != nil {
				ms.logger.Error(err)
			}
			return
		case remove := <-ms.doneChan:
			ms.logger.Debug("Stop called. We are done!")
			err := ms.doneHandler(remove)
			if err != nil {
				ms.logger.Error(err)
			}
			return
		}
	}

}

// Stop - kill the watch time and stop watching
func (ms *MetricsSevrice) Stop(remove bool) {
	ms.doneChan <- remove
}

// SetMetricsHandler - set call back after reading the metrics
func (ms *MetricsSevrice) SetMetricsHandler(erh metrics.ErrorRateHandler) {
	ms.metricsHandler = erh
}

// SetDoneHandler - set the callback handler when watcher is done
func (ms *MetricsSevrice) SetDoneHandler(dh metrics.DoneHandler) {
	ms.doneHandler = dh

}

// SetDoneHandler - set the callback handler when watcher is done
func (ms *MetricsSevrice) SetCheckMetricsFunc(cmf metrics.CheckMetricsFunc) {
	ms.checkMetricsFunc = cmf

}
