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
maxWorkers: 2
accessKeyID: [S3 Access ID]
secretAccessKey: [S3 Secret Key]
bucket: workspace
timerInterval: 15.0
debug: false
~~~

The filelist is a list of filepaths (one file per line), each path in filelist is copied to the same path in the S3 service.

*rootDir* is the path prefix to the files in filelist

*endpoints* is an array of endpoints that should be used in a round robin fashion to upload the data

*maxWorkers* is maximum number for goroutines that will be running to perform uploads

*bucket* is the destination bucket that entries in filelist will be copied to.

*timerInterval* is how often (in seconds) the program outputs status updates.

*accessKeyID* is the S3 accessKeyID (username) that has write access to the S3 *bucket* at the *endpoints*

*secretAccessKey* is the secret key (password) for the accessKeyID

*debug* is a debug flag that enables very detailed logging to the console.

Using the above configuration file and the contents of filelist as:
~~~
f8/f2/f6/f8f2f670-42d4-4c08-80d2-e6e284128226/f8f2f670-42d4-4c08-80d2-e6e284128226.data
1f/73/40/1f7340e8-015b-4a45-a97b-def606b55ef1/1f7340e8-015b-4a45-a97b-def606b55ef1.data
2d/59/e4/2d59e442-daab-460f-ba27-3d55bb5532de/2d59e442-daab-460f-ba27-3d55bb5532de.data
~~~

The the 3 files at the paths:
~~~
/mnt/shock/Shock/data/f8/f2/f6/f8f2f670-42d4-4c08-80d2-e6e284128226/f8f2f670-42d4-4c08-80d2-e6e284128226.data
/mnt/shock/Shock/data/1f/73/40/1f7340e8-015b-4a45-a97b-def606b55ef1/1f7340e8-015b-4a45-a97b-def606b55ef1.data
/mnt/shock/Shock/data/2d/59/e4/2d59e442-daab-460f-ba27-3d55bb5532de/2d59e442-daab-460f-ba27-3d55bb5532de.data
~~~

Would be copied to the following locations using the endpoints listed:
~~~
http://10.58.0.211:7480/workspace/f8/f2/f6/f8f2f670-42d4-4c08-80d2-e6e284128226/f8f2f670-42d4-4c08-80d2-e6e284128226.data
http://10.58.0.212:7480/workspace/1f/73/40/1f7340e8-015b-4a45-a97b-def606b55ef1/1f7340e8-015b-4a45-a97b-def606b55ef1.data
http://10.58.0.211:7480/workspace/2d/59/e4/2d59e442-daab-460f-ba27-3d55bb5532de/2d59e442-daab-460f-ba27-3d55bb5532de.data
~~~

The third file may actually go to either the .211 or the .212 endpoint depending on which upload finished first. Each worker is assigned a single endpoint to send all of it's traffic, so if there are more workers than endpoints then multiple workers will write to the same endpoint. Having more endpoints than workers will result in only the the first few endpoints matching the worker count being used.

It is best to specify a relatively large number of workers ( at least 16, has been tested with up to 96 ) in order to use up the networking bandwidth available. Large files will have higher transfer rate in terms of bytes/sec because the overhead of starting and stopping the transfer is amortized over a longer interval, while smaller files will have higher rate in terms of files/sec, but relatively poor utilization of bandwidth because of the high protocol overhead of starting/stopping the transfer. A large worker pool allows large file transfers to continue in parallel with the many smaller files waiting due to protocol overhead. The KBase shock data is heavily skewed towards small files ( well under 1kb ) which results in an extremely high overhead to payload ratio.

