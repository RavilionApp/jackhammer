package main

type QueueMessage struct {
	// JobID is the ID of the current job, generated and saved in the database.
	JobID string `json:"job_id"`
	// RawURL is the signed S3 URL to the raw (original) user video.
	RawURL string `json:"raw_url"`
	// Key is the location for all output files inside the S3 bucket.
	Key string `json:"key"`
}
