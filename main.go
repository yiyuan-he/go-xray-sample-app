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

func listBuckets(w http.ResponseWriter, r *http.Request) {
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

    // Convert to a simple format for JSON response
    var buckets []map[string]string
    for _, bucket := range result.Buckets {
        bucketInfo := map[string]string{
            "name": *bucket.Name,
            "creation_date": bucket.CreationDate.String(),
        }
        buckets = append(buckets, bucketInfo)
    }

    // Add metadata about the number of buckets to the subsegment
    subseg.AddMetadata("bucket_count", len(buckets))

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(buckets)
}

func main() {
    // Configure X-Ray
    err := xray.Configure(xray.Config{
        DaemonAddr: "127.0.0.1:2000",
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
    http.Handle("/s3-list-buckets", xray.Handler(xray.NewFixedSegmentNamer("go-xray-sample-app"), 
        http.HandlerFunc(listBuckets)))

    // Start server
    fmt.Println("Server starting on :8080")
    if err := http.ListenAndServe(":8080", nil); err != nil {
        log.Fatal(err)
    }
}
