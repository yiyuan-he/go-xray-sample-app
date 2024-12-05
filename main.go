// main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
)

var s3Client *s3.Client

// Manual X-Ray instrumentation
func listBucketsManual(w http.ResponseWriter, r *http.Request) {
	// Get the context from the request which contains X-Ray segment
	ctx := r.Context()

	// Start a subsegment for the S3 Operation
	ctx, subseg := xray.BeginSubsegment(ctx, "ListS3Buckets")
	defer subseg.Close(nil)

	result, err := s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		subseg.AddError(err)
		http.Error(w, fmt.Sprintf("unable to list buckets: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result.Buckets)
}

// AWS SDK auto-instrumentation
func listBucketsAuto(w http.ResponseWriter, r *http.Request) {
	result, err := s3Client.ListBuckets(r.Context(), &s3.ListBucketsInput{})
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to list buckets: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result.Buckets)
}

func main() {
	// Configure X-Ray
	err := xray.Configure(xray.Config{
		DaemonAddr:     "127.0.0.1:2000",
		ServiceVersion: "1.0.0",
	})
	if err != nil {
		log.Fatalf("Failed to configure X-Ray: %v", err)
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion("us-west-2"), // Change this to your desired region
	)
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	// Instrument AWS SDK v2 with X-Ray
	awsv2.AWSV2Instrumentor(&cfg.APIOptions)

	// Create an S3 client
	s3Client = s3.NewFromConfig(cfg)

	// Set up HTTP routes with the X-Ray handler
	http.Handle("/list-buckets-manual",
		xray.Handler(xray.NewFixedSegmentNamer("list-buckets-manual"),
			http.HandlerFunc(listBucketsManual)))

	http.Handle("/list-buckets-auto",
		xray.Handler(xray.NewFixedSegmentNamer("list-buckets-auto"),
			http.HandlerFunc(listBucketsAuto)))

	// Start server
	fmt.Println("Server starting on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
