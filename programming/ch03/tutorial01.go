package ch03

import (
	"context"
	"flag"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	clientSet *kubernetes.Clientset
)

func ListNamespace(ctx context.Context) {
	fmt.Println("start ListNamespace")
	ns, err := clientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Println(err)
	}
	for _, item := range ns.Items {
		fmt.Println("ns name = ", item.Name)
	}
}

func ListPods(ctx context.Context) {
	fmt.Println("start ListPods")
	pods, err := clientSet.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Println(err)
	}
	for _, item := range pods.Items {
		fmt.Println("pod name = ", item.Name)
	}
}

func Watch(ctx context.Context) {
	fmt.Println("start watch")
	watch, err := clientSet.CoreV1().Pods("default").Watch(ctx, metav1.ListOptions{
		LabelSelector: "",
	})
	if err != nil {
		log.Println("error watch")
		panic(err)
	}

	go func() {
		for event := range watch.ResultChan() {
			fmt.Println("labels () has event : ", event.Type)
		}
	}()
}

func CreateInformer() {
	fmt.Println("create informer")
	withNamespace := informers.WithNamespace("default")
	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientSet, time.Second*30, withNamespace)

	podInformer := informerFactory.Core().V1().Pods()
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			log.Println("add pod : ", pod.Name, pod.Namespace)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			log.Println("update pod : ", pod.Name)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*v1.Pod)
			log.Println("delete pod : ", pod.Name)
		},
	})

	informerFactory.Start(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	go podInformer.Informer().Run(wait.NeverStop)
	informerFactory.WaitForCacheSync(wait.NeverStop)

	pods, err := podInformer.Lister().Pods("default").List(labels.NewSelector())
	if err != nil {
		return
	}
	for _, pod := range pods {
		log.Println("informer found pod : ", pod.Name)
	}
}

func init() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	flag.Parse()
	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	// create the clientSet
	clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
