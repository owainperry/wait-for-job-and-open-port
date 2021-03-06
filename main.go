package main

import (
	"context"
	"flag"
	"fmt"
	v1 "k8s.io/api/batch/v1"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"os"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/client-go/rest"

)

type JobHunter struct {
	clientset *kubernetes.Clientset
}

func (j *JobHunter) CompletedJob(namespace string, labelKey string, labelValue string) (int, int, error) {
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{labelKey: labelValue}}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
		Limit:         5,
	}

	listOptions = metav1.ListOptions{}
	

	jobs, err := j.clientset.BatchV1().Jobs(namespace).List(context.TODO(), listOptions)
	if err != nil {
		return 0, 0, err
	}
	log.Printf("Discovered %d jobs with label %s %s \n", len(jobs.Items), labelKey, labelValue)

	jobCompleteCount := 0
	total := len(jobs.Items)
	for i := range jobs.Items {
		for y := range jobs.Items[i].Status.Conditions {
			condition := jobs.Items[i].Status.Conditions[y]
			if condition.Type == v1.JobComplete {
				jobCompleteCount++
			}
		}
	}

	return total, jobCompleteCount, nil
}

func NewJobHunter(clientset *kubernetes.Clientset) JobHunter {
	return JobHunter{
		clientset: clientset,
	}
}

type arrayFlags []string

func (i *arrayFlags) String() string {
	return "my string representation"
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var labelsList arrayFlags

func main() {
	var kubeconfig *string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	var port *string
	port = flag.String("port", "8001", "port to listen on")

	var retries *int
	retries = flag.Int("retries", 600, "number of retries")

	var inCluster *bool
	inCluster = flag.Bool("incluster",true,"use incluster config")

	flag.Var(&labelsList, "labels", "key value pair of lables to find jobs using")

	flag.Parse()

	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	log.Printf("namespace: %s",namespace)

	var config *rest.Config
	var err error

	if *inCluster {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	} else {
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
	}

	// create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	
	for i := 0; i < *retries; i++ {
		jh := NewJobHunter(clientset)
		allComplete := true
		for l := range labelsList {
			fmt.Println()
			bits := strings.Split(labelsList[l], ":")
			total, completed, err := jh.CompletedJob(namespace, bits[0], bits[1])
			log.Printf("jobs with label: %s = %s total jobs: %d completed %d", bits[0], bits[1], total, completed)
			if err != nil {
				log.Printf("Error: %v", err)
				allComplete = false
			}
			if total != completed {
				allComplete = false
			}
		}

		if allComplete {
			break
		}
		time.Sleep(1 * time.Second)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})

	log.Printf("Jobs all complete starting server at port %s", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", *port), nil); err != nil {
		log.Fatal(err)
	}

}
