// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package job

import (
	"log"

	"github.com/kubernetes/dashboard/client"
	"github.com/kubernetes/dashboard/resource/common"
	"github.com/kubernetes/dashboard/resource/pod"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/batch"
	k8sClient "k8s.io/kubernetes/pkg/client/unversioned"
)

// JobDetail is a presentation layer view of Kubernetes Job resource. This means
// it is Job plus additional augmented data we can get from other sources
// (like services that target the same pods).
type JobDetail struct {
	ObjectMeta common.ObjectMeta `json:"objectMeta"`
	TypeMeta   common.TypeMeta   `json:"typeMeta"`

	// Aggregate information about pods belonging to this Job.
	PodInfo common.PodInfo `json:"podInfo"`

	// Detailed information about Pods belonging to this Job.
	PodList pod.PodList `json:"podList"`

	// Container images of the Job.
	ContainerImages []string `json:"containerImages"`

	// List of events related to this Job.
	EventList common.EventList `json:"eventList"`
}

// GetJobDetail gets job details.
func GetJobDetail(client k8sClient.Interface, heapsterClient client.HeapsterClient,
	namespace, name string) (*JobDetail, error) {

	log.Printf("Getting details of %s service in %s namespace", name, namespace)

	// TODO(floreks): Use channels.
	jobData, err := client.Extensions().Jobs(namespace).Get(name)
	if err != nil {
		return nil, err
	}

	channels := &common.ResourceChannels{
		PodList: common.GetPodListChannel(client, common.NewSameNamespaceQuery(namespace), 1),
	}

	pods := <-channels.PodList.List
	if err := <-channels.PodList.Error; err != nil {
		return nil, err
	}

	events, err := GetJobEvents(client, jobData.Namespace, jobData.Name)
	if err != nil {
		return nil, err
	}

	job := getJobDetail(jobData, heapsterClient, events, pods.Items)
	return &job, nil
}

func getJobDetail(job *batch.Job, heapsterClient client.HeapsterClient,
	events *common.EventList, pods []api.Pod) JobDetail {

	matchingPods := common.FilterNamespacedPodsBySelector(pods, job.ObjectMeta.Namespace,
		job.Spec.Selector.MatchLabels)

	podInfo := getPodInfo(job, matchingPods)

	return JobDetail{
		ObjectMeta:      common.NewObjectMeta(job.ObjectMeta),
		TypeMeta:        common.NewTypeMeta(common.ResourceKindJob),
		ContainerImages: common.GetContainerImages(&job.Spec.Template.Spec),
		PodInfo:         podInfo,
		PodList:         pod.CreatePodList(matchingPods, heapsterClient),
		EventList:       *events,
	}
}