# bulkS3upload

## Description

This is a tool written in Go that uses S3 client libraries and Go workers to concurrently upload large numbers of files into an S3 destination.

## Usage

The binary is written in Go and can be built using the standard "go build" command. It is run with a single argument specifying the files to be copied to the S3 endpoint(s).
~~~
./bulkS3upload filelist
~~~

It is configured with a .bulkS3upload.yaml file that looks something like this:

~~~
rootDir: /mnt/shock/Shock/data/
endpoints:
  - 10.58.0.211:7480
  - 10.58.0.212:7480
  - 10.58.0.213:7480
  - 10.58.0.214:7480
  - 10.58.0.215:7480
  - 10.58.0.216:7480
  - 10.58.0.217:7480
  - 10.58.0.218:7480
maxWorkers: 96
accessKeyID: [S3 Access ID]
secretAccessKey: [S3 Secret Key]
bucket: workspace
timerInterval: 15.0
~~~

The filelist is a list of filepaths (one file per line), each path in filelist is copied to the same path in the S3 service.

*rootDir* is the path prefix to the files in filelist

*endpoints* is an array of endpoints that should be used in a round robin fashion to upload the data

*maxWorkers* is maximum number for goroutines that will be running to perform uploads

*bucket* is the destination bucket that entries in filelist will be copied to.

*timerInterval* is how often (in seconds) the program outputs status updates.

*accessKeyID* is the S3 accessKeyID (username) that has write access to the S3 *bucket* at the *endpoints*

*secretAccessKey* is the secret key (password) for the accessKeyID

