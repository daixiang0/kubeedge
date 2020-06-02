package client

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/kubeedge/beehive/pkg/core/model"
	"github.com/kubeedge/kubeedge/edge/pkg/common/message"
	"github.com/kubeedge/kubeedge/edge/pkg/common/modules"
)

//PodsGetter is interface to get pods
type PodsGetter interface {
	Pods(namespace string) PodsInterface
}

//PodsInterface is pod interface
type PodsInterface interface {
	Create(*corev1.Pod) (*corev1.Pod, error)
	Update(*corev1.Pod) error
	Delete(name, options string) error
	Get(name string) (*corev1.Pod, error)
}

type pods struct {
	name string
	send SendInterface
}

func newPods(n string, s SendInterface) *pods {
	return &pods{
		send: s,
		name: n,
	}
}

func (c *pods) Create(cm *corev1.Pod) (*corev1.Pod, error) {
	return nil, nil
}

func (c *pods) Update(cm *corev1.Pod) error {
	return nil
}

func (c *pods) Delete(name, options string) error {
	resource := fmt.Sprintf("%s/%s/%s", c.name, model.ResourceTypePod, name)
	podDeleteMsg := message.BuildMsg(modules.MetaGroup, "", modules.EdgedModuleName, resource, model.DeleteOperation, options)
	c.send.Send(podDeleteMsg)
	return nil
}

func (c *pods) Get(name string) (*corev1.Pod, error) {
	return nil, nil
}
