// Copyright 2020 Chaos Mesh Authors.
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

package errors

var (
	ErrNoNeedSchedule                 = New("no need to schedule")
	ErrNoSuchNode                     = New("no such node")
	ErrNoSuchTemplate                 = New("no such template")
	ErrTemplatesIsRequired             = New("missing required templates in workflow spec")
	ErrTreeNodeIsRequired             = New("missing required tree node in workflow status")
	ErrUnsupportedNodeType            = New("unsupported node type")
	ErrParseSerialTemplateFailed      = New("failed to parse serial template")
	ErrNoMoreTemplateInSerialTemplate = New("no more template could schedule in serial template")
)

type WorkflowError struct {
	Message string
}

func (it *WorkflowError) Error() string {
	return it.Message
}

func New(message string) *WorkflowError {
	return &WorkflowError{
		Message: message,
	}
}
