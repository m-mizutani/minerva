package api

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/athena"
)

type queryStatus string

const (
	statusSuccess queryStatus = "SUCCEEDED"
	statusFail                = "FAILED"
	statusRunning             = "RUNNING"
)

// https://docs.aws.amazon.com/athena/latest/APIReference/API_QueryExecutionStatus.html
// Valid Values: QUEUED | RUNNING | SUCCEEDED | FAILED | CANCELLED
var athenaQueryStatusMap map[string]queryStatus = map[string]queryStatus{
	"QUEUED":    statusRunning, // QUEUED and RUNNING is same from a point of frontend view.
	"RUNNING":   statusRunning,
	"SUCCEEDED": statusSuccess,
	"FAILED":    statusFail,
	"CANCELLED": statusFail, // Minerva does not have cancellation mechanism, then cancel is not normal operation.
}

func toQueryStatus(athenaStatus string) queryStatus {
	status, ok := athenaQueryStatusMap[athenaStatus]
	if !ok {
		Logger.WithField("status", athenaStatus).Fatal("Unsupported Athena query status")
		return "UNKNOWN"
	}

	if athenaStatus == "QUEUED" {
		Logger.Warn("Athena query is queued")
	}

	return status
}

type getQueryExecutionMetaData struct {
	Status         queryStatus `json:"status"`
	SubmittedTime  time.Time   `json:"submitted_time"`
	ElapsedSeconds float64     `json:"elapsed_seconds"`
}

type athenaQueryStatus struct {
	Status      string
	ElapsedTime time.Duration
	OutputPath  string
}

func getAthenaQueryStatus(region, queryID string) (*athenaQueryStatus, Error) {
	ssn := session.Must(session.NewSession(&aws.Config{Region: &region}))
	athenaClient := athena.New(ssn)

	output, err := athenaClient.GetQueryExecution(&athena.GetQueryExecutionInput{
		QueryExecutionId: &queryID,
	})
	if err != nil {
		return nil, wrapSystemError(err, 500, "Fail GetQueryExecution in getQuery")
	}

	status := athenaQueryStatus{}
	if output != nil && output.QueryExecution != nil {
		if output.QueryExecution.Status != nil {
			if s := output.QueryExecution.Status.SubmissionDateTime; s != nil {
				if output.QueryExecution.Status.CompletionDateTime != nil {
					status.ElapsedTime = output.QueryExecution.Status.CompletionDateTime.Sub(*s)
				} else {
					status.ElapsedTime = time.Now().UTC().Sub(*s)
				}
			}

			status.Status = aws.StringValue(output.QueryExecution.Status.State)
		}

		if output.QueryExecution.ResultConfiguration != nil {
			status.OutputPath = aws.StringValue(output.QueryExecution.ResultConfiguration.OutputLocation)
		}
	}

	return &status, nil
}
