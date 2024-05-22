/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Note: the example only works with the code within the same release/branch.
package main

import (
	"flag"
	"path/filepath"

	"github.com/shinemost/k8s-client-go/controller"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/util/workqueue"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
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

	// 创建 k8s Client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	// 从指定的客户端、资源、命名空间和字段选择器创建⼀个新的 List-Watch
	podListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(),
		"pods", v1.NamespaceDefault, fields.Everything())

	// 构造⼀个具有速率限制排队功能的新的 Workqueue
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// 创建 Indexer 和 Informer
	indexer, informer := cache.NewIndexerInformer(podListWatcher, &v1.Pod{},
		0, cache.ResourceEventHandlerFuncs{
			//当有Pod创建时，根据Delta Queue弹出的Object⽣成对应的Key，并加⼊Workqueue中。此处可以根据 Object 的⼀些属性进⾏过滤
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					queue.Add(key)
				}
			},
			//Pod 删除操作
			DeleteFunc: func(obj interface{}) {
				// 在⽣成 Key 之前检查对象。因为资源删除后有可能会进⾏重建等操作，如果监听时错过
				//了删除信息，会导致该条记录是陈旧的
				key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)

				if err == nil {
					queue.Add(key)
				}
			},
		}, cache.Indexers{})

	// 创建新的 Controller
	controller := controller.NewController(queue, indexer, informer)

	stop := make(chan struct{})
	defer close(stop)
	// 启动 Controller
	go controller.Run(1, stop)
	select {}

}
