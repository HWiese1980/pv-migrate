package k8s

import (
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"time"
)

func GetServiceAddress(svc *corev1.Service, kubeClient *kubernetes.Clientset) (string, error) {
	// todo move commented-out logic to cross-cluster rsync code
	//if svc.Spec.Type == corev1.ServiceTypeClusterIP {
	return svc.Name + "." + svc.Namespace, nil
	//}

	//for {
	//	createdService, err := kubeClient.CoreV1().Services(svc.Namespace).Get(context.TODO(), svc.Name, v1.GetOptions{})
	//	if err != nil {
	//		return "", err
	//	}
	//
	//	if len(createdService.Status.LoadBalancer.Ingress) == 0 {
	//		sleepInterval := 10 * time.Second
	//		log.Infof("wait for external ip, sleep %s", sleepInterval)
	//		time.Sleep(sleepInterval)
	//		continue
	//	}
	//	return createdService.Status.LoadBalancer.Ingress[0].IP, nil
	//}
}

func CreateJobWaitTillCompleted(kubeClient *kubernetes.Clientset, job batchv1.Job) error {
	succeeded := make(chan bool)
	defer close(succeeded)
	stopCh := make(chan struct{})
	defer close(stopCh)
	sharedInformerFactory := informers.NewSharedInformerFactory(kubeClient, 5*time.Second)
	sharedInformerFactory.Core().V1().Pods().Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old interface{}, new interface{}) {
				newPod := new.(*corev1.Pod)
				if newPod.Namespace == job.Namespace && newPod.Labels["job-name"] == job.Name {
					switch newPod.Status.Phase {
					case corev1.PodSucceeded:
						log.WithFields(log.Fields{
							"jobName": job.Name,
							"podName": newPod.Name,
						}).Info("Job completed...")
						succeeded <- true
					case corev1.PodRunning:
						log.WithFields(log.Fields{
							"jobName": job.Name,
							"podName": newPod.Name,
						}).Info("Job is running ")
					case corev1.PodFailed, corev1.PodUnknown:
						succeeded <- false
					}
				}
			},
		},
	)

	sharedInformerFactory.Start(stopCh)

	log.WithFields(log.Fields{
		"jobName": job.Name,
	}).Info("Creating rsync job")
	_, err := kubeClient.BatchV1().Jobs(job.Namespace).Create(context.TODO(), &job, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"jobName": job.Name,
	}).Info("Waiting for rsync job to finish")
	if !<-succeeded {
		return errors.New("job did not succeed")
	}
	return nil
}
