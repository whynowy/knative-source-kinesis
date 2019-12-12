/*
Copyright 2019

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

package kinesis

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	ks "github.com/aws/aws-sdk-go/service/kinesis"
	kc "github.com/vmware/vmware-go-kcl/clientlibrary/interfaces"
	"go.uber.org/zap"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink              func(http.ResponseWriter, *http.Request)
		reqBody           string
		expectedEventType string
		error             bool
	}{
		"happy": {
			sink:    sinkAccepted,
			reqBody: `{"CacheEntryTime":null,"CacheExitTime":null,"Records":[{"ApproximateArrivalTimestamp":null,"Data":"eyJrZXkiOiJ2YWx1ZSJ9","EncryptionType":null,"PartitionKey":"1","SequenceNumber":"1234567"}],"Checkpointer":null,"MillisBehindLatest":1000}`,
		},
		"rejected": {
			sink:    sinkRejected,
			reqBody: `{"CacheEntryTime":null,"CacheExitTime":null,"Records":[{"ApproximateArrivalTimestamp":null,"Data":"eyJrZXkiOiJ2YWx1ZSJ9","EncryptionType":null,"PartitionKey":"1","SequenceNumber":"1234567"}],"Checkpointer":null,"MillisBehindLatest":1000}`,
			error:   true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()
			streamArn := "arn:aws:kinesis:us-west-2:4444444:stream/kinesis-name"

			a := &Adapter{
				StreamName:    "kinesis-name",
				Region:        "us-west-2",
				SinkURI:       sinkServer.URL,
				KCLIAMRoleARN: "kiam-role-arn",
				streamARN:     &streamArn,
				ConsumerName:  "source-name",
			}

			if err := a.initClient(); err != nil {
				t.Errorf("failed to create cloudevent client, %v", err)
			}

			data, err := json.Marshal(map[string]string{"key": "value"})
			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			sequenceNumber := "1234567"
			partitionKey := "1"

			record1 := ks.Record{
				Data:           data,
				SequenceNumber: &sequenceNumber,
				PartitionKey:   &partitionKey,
			}

			records := make([]*ks.Record, 1)
			records[0] = &record1

			var checkPointer kc.IRecordProcessorCheckpointer

			m := &kc.ProcessRecordsInput{
				Records:            records,
				Checkpointer:       checkPointer,
				MillisBehindLatest: 1000,
			}
			err = a.postMessage(m, zap.S())

			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}

			et := h.header.Get("Ce-Type")

			expectedEventType := eventType
			if tc.expectedEventType != "" {
				expectedEventType = tc.expectedEventType
			}

			if et != expectedEventType {
				t.Errorf("Expected eventtype %q, but got %q", tc.expectedEventType, et)
			}
			if tc.reqBody != string(h.body) {
				t.Errorf("expected request body %q, but got %q", tc.reqBody, h.body)
			}
		})
	}
}

type fakeHandler struct {
	body   []byte
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.header = r.Header
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.handler(w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}
