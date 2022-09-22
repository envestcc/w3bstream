package vm

import (
	"github.com/google/uuid"
	"github.com/iotexproject/Bumblebee/x/mapx"
	"github.com/iotexproject/w3bstream/pkg/types/wasm"
)

var instances = mapx.New[uint32, wasm.Instance]()

func AddInstance(i wasm.Instance) uint32 {
	id := uuid.New().ID()
	instances.Store(id, i)
	return id
}

func DelInstance(id uint32) error {
	i, _ := instances.LoadAndRemove(id)
	if i != nil && i.State() == wasm.InstanceState_Started {
		i.Stop()
	}
	return nil
}

func StartInstance(id uint32) error {
	return nil
}

func StopInstance(id uint32) error {
	return nil
}

func GetInstanceState(id uint32) (wasm.InstanceState, bool) {
	return wasm.InstanceState_Stopped, true
}