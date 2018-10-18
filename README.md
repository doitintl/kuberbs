# kuberbs 

K8 deployment automatic rollback system

[![License](https://img.shields.io/github/license/doitintl/kuberbs.svg)](LICENSE) [![GitHub stars](https://img.shields.io/github/stars/doitintl/kuberbs.svg?style=social&label=Stars&style=for-the-badge)](https://github.com/doitintl/kuberbs) [![Build Status](https://secure.travis-ci.org/doitintl/kuberbs.png?branch=master)](http://travis-ci.org/doitintl/kuberbs)

[Blog post](https://blog.doit-intl.com/kuberbs-for-automatic-kubernetes-rollbacks-so-you-can-sleep-better-at-night-c119d64590ec)
# Deploy kuberbs (without building from source)

If you just want to use KubeRBS (instead of building it from source yourself), please follow instructions in this section. You need a Kubernetes 1.10 or newer cluster.
You can install Kuberbs on any Kubernetes cluster either on perm or in the cloud.
Kuberbs supports metrics from DataDog or Stackdriver

## GKE Specific Setup  
You'll also need the Google Cloud SDK.
You can install the Google Cloud SDK (which also installs kubectl) [here](https://cloud.google.com/sdk).

Configure gcloud sdk by setting your default project:

```
gcloud config set project {your project_id}
```

Set the environment variables: 
 
```
export GCP_REGION=us-central1
export GKE_CLUSTER_NAME=kuberbs-cluster
export PROJECT_ID=$(gcloud config list --format 'value(core.project)')
```

**Create IAM Service Account and obtain the Key in JSON format**

Create Service Account with this command: 

```
gcloud iam service-accounts create kuberbs-service-account --display-name "kuberbs"
```

Create and attach custom kuberbs role to the service account by running the following commands:

```
gcloud iam roles create kuberbs --project $PROJECT_ID --file roles.yaml

gcloud projects add-iam-policy-binding $PROJECT_ID --member serviceAccount:kuberbs-service-account@$PROJECT_ID.iam.gserviceaccount.com --role projects/$PROJECT_ID/roles/kuberbs
```

Generate the Key using the following command:

```
gcloud iam service-accounts keys create key.json \
--iam-account kuberbs-service-account@$PROJECT_ID.iam.gserviceaccount.com
```
 
**Create Kubernetes Secret**

Get your GKE cluster credentials with (replace *cluster_name* with your real GKE cluster name):

gcloud container clusters get-credentials $GKE_CLUSTER_NAME \
--region $GCP_REGION \
--project $PROJECT_ID
 
## None GKE Specific Setup 
Create a Kubernetes secret by running:

```
kubectl create secret generic kuberbs-key --from-file=key.json --namespace=kube-system
```

**Deploy Kuberbs**

```
kubectl apply -f deploy/.
```

**Create Configuration file**

Change the example in `examples\kuberbs-example.yaml` to fit your needs

Here are the configuration options

  `watchperiod`  int - for how long to watch a deployment
  
  `metricssource`  string - currently we support stackdriver and datadog
  
  for each namespace that you would like to watch you can have multiple deployments.
  Each deployments must have a name, a metric and the threshold per second.

Deploy your configuration file.


#  Deploy & Build From Source


You need a Kubernetes 1.10 or newer cluster. You also need Docker and kubectl 1.10.x or newer installed on your machine, as well as the Google Cloud SDK. You can install the Google Cloud SDK (which also installs kubectl) [here](https://cloud.google.com/sdk).


**Clone Git Repository**

Make sure your $GOPATH is [configured](https://github.com/golang/go/wiki/SettingGOPATH). You'll need to clone this repository to your `$GOPATH/src` folder. 

```
git clone https://github.com/doitintl/kuberbs.git $GOPATH/src/kuberbs
cd $GOPATH/src/kuberbs 
```

**Build kubeRBS's container image**

Install go/dep (Go dependency management tool) using [these instructions](https://github.com/golang/dep) and then run

```
dep ensure
```

You can now compile the kuberbs binary and run tests 

```
make
```

**Build kuberbs's container image**

Compile the kuberbs binary and build the Docker image as following:


```
make image
```

Tag the image using: 

```
docker tag kuberbs gcr.io/$PROJECT_ID/kuberbs
```

Finally, push the image to  Container Registry with: 

For example
```
docker push gcr.io/$PROJECT_ID/kuberbs
```
## GKE Specific Setup 
 
**Set Environment Variables**

Replace **us-central1** with the region where your GKE cluster resides and **kuberbs-cluster** with your real GKE cluster name

```
export GCP_REGION=us-central1
export GKE_CLUSTER_NAME=kuberbs-cluster
export PROJECT_ID=$(gcloud config list --format 'value(core.project)')
```

**Create IAM Service Account and obtain the Key in JSON format**

Create Service Account with this command: 

```
gcloud iam service-accounts create kuberbs-service-account --display-name "kuberbs"
```

Create and attach custom kuberbs role to the service account by running the following commands:

```
gcloud iam roles create kuberbs --project $PROJECT_ID --file roles.yaml

gcloud projects add-iam-policy-binding $PROJECT_ID --member serviceAccount:kuberbs-service-account@$PROJECT_ID.iam.gserviceaccount.com --role projects/$PROJECT_ID/roles/kuberbs
```

Generate the Key using the following command:

```
gcloud iam service-accounts keys create key.json \
--iam-account kuberbs-service-account@$PROJECT_ID.iam.gserviceaccount.com
```
 
**Create Kubernetes Secret**

Get your GKE cluster credentials with (replace *cluster_name* with your real GKE cluster name):

gcloud container clusters get-credentials $GKE_CLUSTER_NAME \
--region $GCP_REGION \
--project $PROJECT_ID
 
## None GKE Specific Setup 

Create a Kubernetes secret by running:

```
kubectl create secret generic kuberbs-key --from-file=key.json --namespace=kube-system
```

Deploy kuberbs by running

kubectl apply -f deploy/.

**Running kuberbs locally**

Make sure you can access your cluster

`APISERVER=$(kubectl config view --minify | grep server | cut -f 2- -d ":" | tr -d " ")`

`TOKEN=$(kubectl describe secret $(kubectl get secrets | grep ^default | cut -f1 -d ' ') | grep -E '^token' | cut -f2 -d':' | tr -d " ")`

`curl $APISERVER/api --header "Authorization: Bearer $TOKEN" --insecure`

If you can access your cluster then you need to set up the credentials:

`echo $TOKEN >token`
`tr -d '\n' <token >t`
`sudo cp t /var/run/secrets/kubernetes.io/serviceaccount/token`
ssh into one of the containers in your cluster and get `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`. copy that file to your local `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt`

In the file `pkg/clinetset/v1/rbs` 
replace `const RbsNameSpace = "kube-system"` with `const RbsNameSpace = "default"` 

References:

Event listening code was taken from [kubewatch](https://github.com/bitnami-labs/kubewatch/)
