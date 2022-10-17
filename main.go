package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"reflect"

	"github.com/kubernetes/kompose/pkg/kobject"
	"github.com/kubernetes/kompose/pkg/loader"
	"github.com/kubernetes/kompose/pkg/transformer/kubernetes"

	//"k8s.io/apiserver/pkg/admission/plugin/webhook/namespace"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getDefaultNs() (string, error) {
	clientCfg, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return "", err
	}
	namespace := clientCfg.Contexts[clientCfg.CurrentContext].Namespace

	if namespace == "" {
		namespace = "default"
	}
	return namespace, nil
}

// TODO pick the defautnamespace from the kubeconfig file

// add complex stuff like volumes

// test compose files - automate that process of running the main file

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kube.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	namespace, err := getDefaultNs()
	if err != nil {
		error := fmt.Errorf("[error] Can't get current namespace for the context %s", err.Error())
		fmt.Println(error)
		return
	}

	l, err := loader.GetLoader("compose")
	if err != nil {
		log.Fatal(err)
	}

	komposeObj := kobject.KomposeObject{
		ServiceConfigs: make(map[string]kobject.ServiceConfig),
	}

	opt := kobject.ConvertOptions{
		CreateD:                true,
		CreateDeploymentConfig: true,
		Volumes:                "hostPath",
		Replicas:               1,
		Provider:               "kubernetes",
		InputFiles:             []string{"compose.yml"},
	}

	komposeObj, err = l.LoadFile(opt.InputFiles)

	if err != nil {
		log.Fatal(err)
	}

	t := &kubernetes.Kubernetes{Opt: opt}

	objects, err := t.Transform(komposeObj, opt)

	if err != nil {
		log.Fatal(err)
	}

	deploymentsClient := clientset.AppsV1().Deployments(namespace)

	// dep :=  objects[2].(*appsv1.Deployment)
	// fmt.Printf("%v\n", dep)

	deploymentObjs := make([]*appsv1.Deployment, 0)
	for i := 0; i < len(objects); i++ {
		if objects[i].GetObjectKind().GroupVersionKind().Kind == "Deployment" {
			depl := objects[i].(*appsv1.Deployment)
			deploymentObjs = append(deploymentObjs, depl)
		}
	}

	for i := 0; i < len(deploymentObjs); i++ {
		result, err := deploymentsClient.Create(context.TODO(), deploymentObjs[i], metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		fmt.Printf("Create Deployment %q.\n", result.GetObjectMeta().GetName())
	}

	// creating service objects
	keys := reflect.ValueOf(komposeObj.ServiceConfigs).MapKeys()
	serviceObjs := make([]*apiv1.Service, 0)
	for _, key := range keys {
		svc := *t.CreateService(key.Interface().(string), komposeObj.ServiceConfigs[key.Interface().(string)])
		serviceObjs = append(serviceObjs, &svc)
	}

	servicesClient := clientset.CoreV1().Services(namespace)

	for i := 0; i < len(serviceObjs); i++ {
		result, err := servicesClient.Create(context.TODO(), serviceObjs[i], metav1.CreateOptions{})
		if err != nil {
			panic(err)
		}
		fmt.Printf("Create Service %q.\n", result.GetObjectMeta().GetName())
	}
}