/*
Copyright 2018 The KubeEdge Authors.

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

package metamanager

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/kubeedge/beehive/pkg/core"
	beehiveContext "github.com/kubeedge/beehive/pkg/core/context"
	"github.com/kubeedge/beehive/pkg/core/model"
	"github.com/kubeedge/kubeedge/edge/mocks/beego"
	connect "github.com/kubeedge/kubeedge/edge/pkg/common/cloudconnection"
	"github.com/kubeedge/kubeedge/edge/pkg/common/dbm"
	"github.com/kubeedge/kubeedge/edge/pkg/common/modules"
	"github.com/kubeedge/kubeedge/edge/pkg/metamanager/dao"
)

const (
	// FailedDBOperation is common Database operation fail message
	FailedDBOperation = "failed to operate DB"
	// ModuleNameEdged is name of edged module
	ModuleNameEdged = "edged"
	// ModuleNameEdgeHub is name of edgehub module
	ModuleNameEdgeHub = "websocket"
	// ModuleNameController is the name of the controller module
	ModuleNameController = "edgecontroller"
	// MarshalErroris common jsonMarshall error
	MarshalError = "Error to marshal message content: json: unsupported type: chan int"
	// OperationNodeConnection is message with operation publish
	OperationNodeConnection = "publish"
)

// errFailedDBOperation is common Database operation fail error
var errFailedDBOperation = errors.New(FailedDBOperation)

// metaMgrModule is metamanager implementation of Module interface
var metaMgrModule core.Module

// TestProcessInsert is function to test processInsert
func TestProcessInsert(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ormerMock := beego.NewMockOrmer(mockCtrl)
	querySeterMock := beego.NewMockQuerySeter(mockCtrl)
	dbm.DBAccess = ormerMock

	core.Register(&metaManager{enable: true})
	for name, module := range core.GetModules() {
		if name == MetaManagerModuleName {
			metaMgrModule = module
			break
		}
	}

	t.Run("ModuleRegistration", func(t *testing.T) {
		if metaMgrModule == nil {
			t.Errorf("MetaManager Module not Registered with beehive core")
		}
	})
	beehiveContext.AddModule(metaMgrModule.Name())
	beehiveContext.AddModuleGroup(metaMgrModule.Name(), metaMgrModule.Group())

	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), nil).Times(1)

	metaMgrModule.Start()

	//SaveMeta Failed, feedbackError SendToCloud
	ormerMock.EXPECT().Insert(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	msg := model.NewMessage("").BuildRouter(MetaManagerModuleName, GroupResource, model.ResourceTypePodStatus, model.InsertOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, err := beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("EdgeHubChannelRegistration", func(t *testing.T) {
		if err != nil {
			t.Errorf("EdgeHub Channel not found: %v", err)
			return
		}
		want := "Error to save meta to DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//SaveMeta Failed, feedbackError SendToEdged and 2 resources
	ormerMock.EXPECT().Insert(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus+"/secondRes", model.InsertOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, err = beehiveContext.Receive(ModuleNameEdged)
	t.Run("ErrorMessageToEdged", func(t *testing.T) {
		if err != nil {
			t.Errorf("EdgeD Channel not found: %v", err)
			return
		}
		want := "Error to save meta to DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//jsonMarshall fail
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, model.InsertOperation).FillBody(make(chan int))
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("MarshallFail", func(t *testing.T) {
		want := MarshalError
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Succesful Case and 3 resources
	ormerMock.EXPECT().Insert(gomock.Any()).Return(int64(1), nil).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus+"/secondRes"+"/thirdRes", model.InsertOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("InsertMessageToEdged", func(t *testing.T) {
		want := model.InsertOperation
		if message.GetOperation() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetOperation())
		}
	})
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("ResponseMessageToEdgeHub", func(t *testing.T) {
		want := OK
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})
}

// TestProcessUpdate is function to test processUpdate
func TestProcessUpdate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	querySeterMock := beego.NewMockQuerySeter(mockCtrl)
	rawSeterMock := beego.NewMockRawSeter(mockCtrl)
	dbm.DBAccess = ormerMock

	//jsonMarshall fail
	msg := model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, model.UpdateOperation).FillBody(make(chan int))
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ := beehiveContext.Receive(ModuleNameEdged)
	t.Run("MarshallFail", func(t *testing.T) {
		want := MarshalError
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Database save error
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, errFailedDBOperation).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, model.UpdateOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("DatabaseSaveError", func(t *testing.T) {
		want := "Error to update meta to DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//resourceUnchanged true
	fakeDao := new([]dao.Meta)
	fakeDaoArray := make([]dao.Meta, 1)
	fakeDaoArray[0] = dao.Meta{Key: "Test", Value: "\"test\""}
	fakeDao = &fakeDaoArray
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, "test/"+model.ResourceTypePodStatus, model.UpdateOperation).FillBody("test")
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("ResourceUnchangedTrue", func(t *testing.T) {
		want := OK
		if message.GetContent() != want {
			t.Errorf("Resource Unchanged Case Failed: Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Success Case Source Edged, sync = true
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, nil).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, model.UpdateOperation)
	msg.Header.Sync = true
	message, err := beehiveContext.SendSync(MetaManagerModuleName, *msg, time.Duration(3)*time.Second)
	t.Run("SuccessSourceSyncErrorCheck", func(t *testing.T) {
		if err != nil {
			t.Errorf("Send Sync Failed with error %v", err)
		}
	})
	edgehubMsg, _ := beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("SuccessSourceEdgedReceiveEdgehub", func(t *testing.T) {
		want := model.UpdateOperation
		if edgehubMsg.GetOperation() != want {
			t.Errorf("Wrong message received : Wanted operation %v and Got operation %v", want, edgehubMsg.GetOperation())
		}
	})
	t.Run("SuccessSourceEdgedReceiveEdged", func(t *testing.T) {
		want := OK
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Success Case Source CloudControlerModel
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, nil).Times(1)
	msg = model.NewMessage("").BuildRouter(CloudControlerModel, GroupResource, model.ResourceTypePodStatus, model.UpdateOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("SuccessSend[CloudController->Edged]", func(t *testing.T) {
		want := CloudControlerModel
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted from source %v and Got from source %v", want, message.GetSource())
		}
	})
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("SuccessSendCloud[CloudController->EdgeHub]", func(t *testing.T) {
		want := OK
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Success Case Source CloudFunctionModel
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, nil).Times(1)
	msg = model.NewMessage("").BuildRouter(CloudFunctionModel, GroupResource, model.ResourceTypePodStatus, model.UpdateOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(EdgeFunctionModel)
	t.Run("SuccessSend[CloudFunction->EdgeFunction]", func(t *testing.T) {
		want := CloudFunctionModel
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted from source %v and Got from source %v", want, message.GetSource())
		}
	})

	//Success Case Source EdgeFunctionModel
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, nil).Times(1)
	msg = model.NewMessage("").BuildRouter(EdgeFunctionModel, GroupResource, model.ResourceTypePodStatus, model.UpdateOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("SuccessSend[EdgeFunction->EdgeHub]", func(t *testing.T) {
		want := EdgeFunctionModel
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted from source %v and Got from source %v", want, message.GetSource())
		}
	})
}

// TestProcessResponse is function to test processResponse
func TestProcessResponse(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	rawSeterMock := beego.NewMockRawSeter(mockCtrl)
	dbm.DBAccess = ormerMock

	//jsonMarshall fail
	msg := model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, model.ResponseOperation).FillBody(make(chan int))
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ := beehiveContext.Receive(ModuleNameEdged)
	t.Run("MarshallFail", func(t *testing.T) {
		want := MarshalError
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Database save error
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, errFailedDBOperation).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, model.ResponseOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("DatabaseSaveError", func(t *testing.T) {
		want := "Error to update meta to DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Success Case Source EdgeD
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, nil).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, model.ResponseOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("SuccessSourceEdged", func(t *testing.T) {
		want := ModuleNameEdged
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted from source %v and Got from source %v", want, message.GetSource())
		}
	})

	//Success Case Source EdgeHub
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	rawSeterMock.EXPECT().Exec().Return(nil, nil).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameController, GroupResource, model.ResourceTypePodStatus, model.ResponseOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("SuccessSourceEdgeHub", func(t *testing.T) {
		want := ModuleNameController
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted from source %v and Got from source %v", want, message.GetSource())
		}
	})
}

// TestProcessDelete is function to test processDelete
func TestProcessDelete(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	querySeterMock := beego.NewMockQuerySeter(mockCtrl)
	dbm.DBAccess = ormerMock

	//Database Save Error
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().Delete().Return(int64(1), errFailedDBOperation).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg := model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, model.DeleteOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ := beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("DatabaseDeleteError", func(t *testing.T) {
		want := "Error to delete meta to DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Success Case
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().Delete().Return(int64(1), nil).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, model.DeleteOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("SuccessSourceEdgeHub", func(t *testing.T) {
		want := ModuleNameEdgeHub
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted from source %v and Got from source %v", want, message.GetSource())
		}
	})
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("SuccessResponseOK", func(t *testing.T) {
		want := OK
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})
}

// TestProcessQuery is function to test processQuery
func TestProcessQuery(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	querySeterMock := beego.NewMockQuerySeter(mockCtrl)
	rawSeterMock := beego.NewMockRawSeter(mockCtrl)
	dbm.DBAccess = ormerMock

	//process remote query sync error case
	msg := model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, model.ResourceTypePodStatus, OperationNodeConnection).FillBody(connect.CloudConnected)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	//wait for message to be received by metaManager and get processed
	time.Sleep(1 * time.Second)

	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdged, GroupResource, "test/"+model.ResourceTypeConfigmap, model.QueryOperation)
	beehiveContext.Cleanup(ModuleNameEdgeHub)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ := beehiveContext.Receive(ModuleNameEdged)
	t.Run("ProcessRemoteQuerySyncErrorCase", func(t *testing.T) {
		want := "Error to query meta in DB: bad request module name(websocket)"
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})
	beehiveContext.AddModule(ModuleNameEdgeHub)
	beehiveContext.AddModuleGroup(ModuleNameEdgeHub, modules.HubGroup)

	//process remote query jsonMarshall error
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, "test/"+model.ResourceTypeConfigmap, model.QueryOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	msg = model.NewMessage(message.GetID()).BuildRouter(ModuleNameEdgeHub, GroupResource, "test/"+model.ResourceTypeConfigmap, model.QueryOperation).FillBody(make(chan int))
	beehiveContext.SendResp(*msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("ProcessRemoteQueryMarshallFail", func(t *testing.T) {
		want := MarshalError
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//process remote query db fail
	rawSeterMock.EXPECT().Exec().Return(nil, errFailedDBOperation).Times(1)
	ormerMock.EXPECT().Raw(gomock.Any(), gomock.Any()).Return(rawSeterMock).Times(1)
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, "test/"+model.ResourceTypeConfigmap, model.QueryOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	msg = model.NewMessage(message.GetID()).BuildRouter(ModuleNameEdgeHub, GroupResource, "test/"+model.ResourceTypeConfigmap, model.QueryOperation).FillBody("TestMessage")
	beehiveContext.SendResp(*msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("ProcessRemoteQueryDbFail", func(t *testing.T) {
		want := "TestMessage"
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//No error and connected true
	fakeDao := new([]dao.Meta)
	fakeDaoArray := make([]dao.Meta, 1)
	fakeDaoArray[0] = dao.Meta{Key: "Test", Value: "Test"}
	fakeDao = &fakeDaoArray
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, "test/"+model.ResourceTypeConfigmap, model.QueryOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("DatabaseNoErrorAndMetaFound", func(t *testing.T) {
		want := make([]string, 1)
		want[0] = "Test"
		bytesWant, _ := json.Marshal(want)
		bytesGot, _ := json.Marshal(message.GetContent())
		if string(bytesGot) != string(bytesWant) {
			t.Errorf("Wrong message receive : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//ResId Nil database error
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, OperationNodeConnection).FillBody(connect.CloudDisconnected)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	time.Sleep(1 * time.Second)

	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypeConfigmap, model.QueryOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("ResIDNilDatabaseError", func(t *testing.T) {
		want := "Error to query meta in DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong message receive : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//ResID not nil database error
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, "test/test/"+model.ResourceTypeConfigmap, model.QueryOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("ResIDNotNilDatabaseError", func(t *testing.T) {
		want := "Error to query meta in DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong message receive : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//ResID not nil Success Case
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, "test/test/"+model.ResourceTypeConfigmap, model.QueryOperation)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdged)
	t.Run("DatabaseNoErrorAndMetaFound", func(t *testing.T) {
		want := make([]string, 1)
		want[0] = "Test"
		bytesWant, _ := json.Marshal(want)
		bytesGot, _ := json.Marshal(message.GetContent())
		if string(bytesGot) != string(bytesWant) {
			t.Errorf("Wrong message receive : Wanted %v and Got %v", want, message.GetContent())
		}
	})
}

// TestProcessNodeConnection is function to test processNodeConnection
func TestProcessNodeConnection(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	dbm.DBAccess = ormerMock

	//connected true
	msg := model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, OperationNodeConnection).FillBody(connect.CloudConnected)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	//wait for message to be received by metaManager and get processed
	time.Sleep(1 * time.Second)

	//connected false
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, OperationNodeConnection).FillBody(connect.CloudDisconnected)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	//wait for message to be received by metaManager and get processed
}

// TestProcessSync is function to test processSync
func TestProcessSync(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	querySeterMock := beego.NewMockQuerySeter(mockCtrl)
	dbm.DBAccess = ormerMock

	//QueryAllMeta Error
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	msg := model.NewMessage("").BuildRouter(MetaManagerModuleName, GroupResource, model.ResourceTypePodStatus, OperationMetaSync)
	beehiveContext.Send(MetaManagerModuleName, *msg)

	//Length 0
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	beehiveContext.Send(MetaManagerModuleName, *msg)

	//QueryMetaError
	fakeDao := new([]dao.Meta)
	fakeDaoArray := make([]dao.Meta, 1)
	fakeDaoArray[0] = dao.Meta{Key: "Test/Test/Test", Value: "Test"}
	fakeDao = &fakeDaoArray
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	beehiveContext.Send(MetaManagerModuleName, *msg)

	//QueryMeta Length 0
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().All(gomock.Any()).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().Delete().Return(int64(1), errFailedDBOperation).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ := beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("QueryMetaLengthZero", func(t *testing.T) {
		want := make([]interface{}, 0)
		bytesWant, _ := json.Marshal(want)
		bytesGot, _ := json.Marshal(message.GetContent())
		if string(bytesGot) != string(bytesWant) {
			t.Errorf("Wrong message receive : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//QueryMeta Length > 0 UnMarshalError
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("QueryMetaLengthMoreThanZeroUnmarshalError", func(t *testing.T) {
		want := make([]interface{}, 0)
		bytesWant, _ := json.Marshal(want)
		bytesGot, _ := json.Marshal(message.GetContent())
		if string(bytesGot) != string(bytesWant) {
			t.Errorf("Wrong message receive : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//QueryMeta Length > 0 Success Case
	fakeDaoArray[0] = dao.Meta{Key: "Test/Test/Test", Value: "\"Test\""}
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	querySeterMock.EXPECT().All(gomock.Any()).SetArg(0, *fakeDao).Return(int64(1), nil).Times(1)
	querySeterMock.EXPECT().Filter(gomock.Any(), gomock.Any()).Return(querySeterMock).Times(1)
	ormerMock.EXPECT().QueryTable(gomock.Any()).Return(querySeterMock).Times(1)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("QueryMetaLengthMoreThanZeroUnmarshalError", func(t *testing.T) {
		want := make([]interface{}, 1)
		want[0] = "Test"
		bytesWant, _ := json.Marshal(want)
		bytesGot, _ := json.Marshal(message.GetContent())
		if string(bytesGot) != string(bytesWant) {
			t.Errorf("Wrong message receive : Wanted %v and Got %v", want, message.GetContent())
		}
	})
}

// TestProcessFunctionAction is function to test processFunctionAction
func TestProcessFunctionAction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	dbm.DBAccess = ormerMock

	//jsonMarshall fail
	msg := model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, OperationFunctionAction).FillBody(make(chan int))
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ := beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("MarshallFail", func(t *testing.T) {
		want := MarshalError
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Database Save Error
	ormerMock.EXPECT().Insert(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, OperationFunctionAction)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("DatabaseSaveError", func(t *testing.T) {
		want := "Error to save meta to DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Success Case
	ormerMock.EXPECT().Insert(gomock.Any()).Return(int64(1), nil).Times(1)
	msg = model.NewMessage("").BuildRouter(ModuleNameEdgeHub, GroupResource, model.ResourceTypePodStatus, OperationFunctionAction)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(EdgeFunctionModel)
	t.Run("SuccessCase", func(t *testing.T) {
		want := ModuleNameEdgeHub
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted from source %v and Got from source %v", want, message.GetSource())
		}
	})
}

// TestProcessFunctionActionResult is function to test processFunctionActionResult
func TestProcessFunctionActionResult(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	ormerMock := beego.NewMockOrmer(mockCtrl)
	dbm.DBAccess = ormerMock

	//jsonMarshall fail
	msg := model.NewMessage("").BuildRouter(EdgeFunctionModel, GroupResource, model.ResourceTypePodStatus, OperationFunctionActionResult).FillBody(make(chan int))
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ := beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("MarshallFail", func(t *testing.T) {
		want := MarshalError
		if message.GetContent() != want {
			t.Errorf("Wrong Error message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Database Save Error
	ormerMock.EXPECT().Insert(gomock.Any()).Return(int64(1), errFailedDBOperation).Times(1)
	msg = model.NewMessage("").BuildRouter(EdgeFunctionModel, GroupResource, model.ResourceTypePodStatus, OperationFunctionActionResult)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("DatabaseSaveError", func(t *testing.T) {
		want := "Error to save meta to DB: " + FailedDBOperation
		if message.GetContent() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetContent())
		}
	})

	//Success Case
	ormerMock.EXPECT().Insert(gomock.Any()).Return(int64(1), nil).Times(1)
	msg = model.NewMessage("").BuildRouter(EdgeFunctionModel, GroupResource, model.ResourceTypePodStatus, OperationFunctionActionResult)
	beehiveContext.Send(MetaManagerModuleName, *msg)
	message, _ = beehiveContext.Receive(ModuleNameEdgeHub)
	t.Run("SuccessCase", func(t *testing.T) {
		want := EdgeFunctionModel
		if message.GetSource() != want {
			t.Errorf("Wrong message received : Wanted %v and Got %v", want, message.GetSource())
		}
	})

	// CleanUp after Testing
	beehiveContext.Cleanup(ModuleNameEdgeHub)
}
