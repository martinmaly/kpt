// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

import (
	"errors"
	"fmt"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// Creates an error object from arguments.
func E(args ...interface{}) error {
	if len(args) == 0 {
		klog.Fatalf("no arguments passed to errors.E")
	}
	e := &Error{}
	for _, arg := range args {
		switch a := arg.(type) {
		case Reason:
			e.Reason = a
		case string:
			e.Message = a
		case error:
			e.Err = a
		default:
			// We need generics :)
			klog.Fatalf("unknown type %T for value %v in call to errors.E", a, a)
		}
	}

	// Compress away empty errors
	if wrapped, ok := e.Err.(*Error); ok && wrapped.IsRedundant() {
		e.Err = wrapped.Err
	}
	return e
}

// Error is modeled after kpt's error handling and adjusted
// to support errors encountered in a service.
type Error struct {
	// Reason of the error.
	Reason Reason

	// Message contains an error message.
	Message string

	// Err is a wrapped error, if any.
	Err error
}

// Reason code for the error.
type Reason int

const (
	UnknownReason Reason = iota // Unknown error
	Invalid
	Unauthorized
	NotFound
	Conflict
	Internal
	NotImplemented
)

func (r Reason) String() string {
	switch r {
	default:
		return fmt.Sprintf("unknown (%d)", r)
	case UnknownReason:
		return "unknown"
	case Invalid:
		return "invalid"
	case Unauthorized:
		return "unauthorized"
	case NotFound:
		return "not found"
	case Conflict:
		return "conflict"
	case Internal:
		return "internal"
	case NotImplemented:
		return "not implemented"
	}
}

func (r Reason) StatusReason() metav1.StatusReason {
	switch r {
	default:
		fallthrough
	case UnknownReason:
		return metav1.StatusReasonInternalError
	case Invalid:
		return metav1.StatusReasonBadRequest
	case Unauthorized:
		return metav1.StatusReasonUnauthorized
	case NotFound:
		return metav1.StatusReasonNotFound
	case Conflict:
		return metav1.StatusReasonConflict
	case Internal:
		return metav1.StatusReasonInternalError
	case NotImplemented:
		return metav1.StatusReasonInternalError
	}
}

func (r Reason) Code() int32 {
	switch r {
	default:
		fallthrough
	case UnknownReason:
		return http.StatusInternalServerError
	case Invalid:
		return http.StatusBadRequest
	case Unauthorized:
		return http.StatusUnauthorized
	case NotFound:
		return http.StatusNotFound
	case Conflict:
		return http.StatusConflict
	case Internal:
		return http.StatusInternalServerError
	case NotImplemented:
		return http.StatusNotImplemented
	}
}

var _ error = &Error{}

func (e *Error) Error() string {
	return fmt.Sprintf("%s; %s", e.Reason, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) Status() metav1.Status {
	return metav1.Status{
		Status:  metav1.StatusFailure,
		Message: e.Message,
		Reason:  e.Reason.StatusReason(),
		Details: e.StatusDetails(),
		Code:    e.Reason.Code(),
	}
}

func (e *Error) StatusDetails() *metav1.StatusDetails {
	var causes []metav1.StatusCause

	for err := e.Err; err != nil; err = errors.Unwrap(err) {
		causes = append(causes, metav1.StatusCause{
			Message: err.Error(),
		})
	}

	if len(causes) > 0 {
		return &metav1.StatusDetails{
			Causes: causes,
		}
	}
	return nil
}

func (e *Error) IsRedundant() bool {
	return e.Message == "" && e.Reason == UnknownReason
}
