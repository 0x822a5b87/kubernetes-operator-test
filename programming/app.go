package main

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"os"
	"programming/ch03"
)

func main() {
	ctx := context.Background()
	ch03.ListNamespace(ctx)
	ch03.ListPods(ctx)
	ch03.Watch(ctx)
	ch03.CreateInformer()

	<-wait.NeverStop
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
