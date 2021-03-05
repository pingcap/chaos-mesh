// Copyright Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by MockGen. DO NOT EDIT.
// Source: ./model/template/suspend.go

// Package mock_template is a generated GoMock package.
package mock_template

import (
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"

	template "github.com/chaos-mesh/chaos-mesh/pkg/workflow/model/template"
)

// MockSuspendTemplate is a mock of SuspendTemplate interface.
type MockSuspendTemplate struct {
	ctrl     *gomock.Controller
	recorder *MockSuspendTemplateMockRecorder
}

// MockSuspendTemplateMockRecorder is the mock recorder for MockSuspendTemplate.
type MockSuspendTemplateMockRecorder struct {
	mock *MockSuspendTemplate
}

// NewMockSuspendTemplate creates a new mock instance.
func NewMockSuspendTemplate(ctrl *gomock.Controller) *MockSuspendTemplate {
	mock := &MockSuspendTemplate{ctrl: ctrl}
	mock.recorder = &MockSuspendTemplateMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSuspendTemplate) EXPECT() *MockSuspendTemplateMockRecorder {
	return m.recorder
}

// Duration mocks base method.
func (m *MockSuspendTemplate) Duration() (time.Duration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Duration")
	ret0, _ := ret[0].(time.Duration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Duration indicates an expected call of Duration.
func (mr *MockSuspendTemplateMockRecorder) Duration() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Duration", reflect.TypeOf((*MockSuspendTemplate)(nil).Duration))
}

// Name mocks base method.
func (m *MockSuspendTemplate) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockSuspendTemplateMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockSuspendTemplate)(nil).Name))
}

// TemplateType mocks base method.
func (m *MockSuspendTemplate) TemplateType() template.TemplateType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TemplateType")
	ret0, _ := ret[0].(template.TemplateType)
	return ret0
}

// TemplateType indicates an expected call of TemplateType.
func (mr *MockSuspendTemplateMockRecorder) TemplateType() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TemplateType", reflect.TypeOf((*MockSuspendTemplate)(nil).TemplateType))
}
