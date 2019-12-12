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

package main

import (
	"flag"
	"log"
	"os"

	kinesis "github.com/whynowy/knative-source-kinesis/pkg/adapter"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"golang.org/x/net/context"
)

const (
	// envCredsFile is the path of the AWS credentials file
	envCredsFile = "AWS_APPLICATION_CREDENTIALS"

	// Environment variable containing IAM role ARN to access Kinesis Consumer Library
	envKclIamRoleArn = "KCL_IAM_ROLE_ARN"

	// Environment variable containing stream name
	envStreamName = "STREAM_NAME"

	// Environment variable containing stream region
	envRegion = "REGION"

	// Sink for messages.
	envSinkURI = "SINK_URI"

	// Environment variable for Consumer Name
	envConsumerName = "CONSUMER_NAME"
)

func getRequiredEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		log.Fatalf("required environment variable not defined '%s'", envKey)
	}
	return val
}

func getOptionalEnv(envKey string) string {
	val, defined := os.LookupEnv(envKey)
	if !defined {
		return ""
	}
	return val
}

func main() {
	flag.Parse()

	ctx := context.Background()
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := logCfg.Build()
	if err != nil {
		log.Fatalf("Unable to create logger: %v", err)
	}

	adapter := &kinesis.Adapter{
		KCLIAMRoleARN: getOptionalEnv(envKclIamRoleArn),
		CredsFile:     getOptionalEnv(envCredsFile),
		StreamName:    getRequiredEnv(envStreamName),
		Region:        getRequiredEnv(envRegion),
		SinkURI:       getRequiredEnv(envSinkURI),
		ConsumerName:  getRequiredEnv(envConsumerName),
	}

	logger.Info("Starting Kinesis Receive Adapter.", zap.Any("adapter", adapter))
	stopCh := signals.SetupSignalHandler()
	if err := adapter.Start(ctx, stopCh); err != nil {
		logger.Fatal("failed to start adapter: ", zap.Error(err))
	}
}
