# ibmcloud-volume-file-vpc

[![Build Status](https://api.travis-ci.com/IBM/ibmcloud-volume-file-vpc.svg?branch=master)](https://travis-ci.com/IBM/ibmcloud-volume-file-vpc)
[![Coverage](https://github.com/IBM/ibmcloud-volume-file-vpc/blob/gh-pages/coverage/master/badge.svg)](https://github.com/IBM/ibmcloud-volume-file-vpc/tree/gh-pages/coverage/master/cover.html)


This is an implementation of code which is used in file storage from vpc providers for IBM Cloud Kubernetes Service and Red Hat OpenShift on IBM Cloud

# Build the library

For building the library `GO` should be installed on the system

1. On your local machine, install [`Go`](https://golang.org/doc/install).
2. GO version should be >=1.16
3. Set the [`GOPATH` environment variable](https://github.com/golang/go/wiki/SettingGOPATH).
4. Build the library

   ## Clone the repo or your forked repo

   ```
   $ mkdir -p $GOPATH/src/github.com/IBM
   $ cd $GOPATH/src/github.com/IBM/
   $ git clone https://github.com/IBM/ibmcloud-volume-file-vpc.git
   $ cd ibmcloud-volume-file-vpc
   ```
   ## Build project and runs testcases

   ```
   $ make

   ```
   ## Build test program

   ```
   $ cd cd samples
   $ go build

   ```

## Testing

- Test the all the possible operations via sample program this will act as RIAAS client
  - `cd samples`
  - `./samples`

# E2E Tests

Please follow the detail steps [ here ](https://github.com/IBM/ibmcloud-storage-volume-lib/blob/samdev/e2e-final/README.md)

# How to contribute

If you have any questions or issues you can create a new issue [ here ](https://github.com/IBM/ibmcloud-volume-file-vpc/issues/new).

Pull requests are very welcome! Make sure your patches are well tested. Ideally create a topic branch for every separate change you make. For example:

1. Fork the repo

2. Create your feature branch (git checkout -b my-new-feature)

3. Commit your changes (git commit -am 'Added some feature')

4. Push to the branch (git push origin my-new-feature)

5. Create new Pull Request

6. Add the test results in the PR


# For more details on support of CLI and VPC IAAS layer please refer below documentation
https://cloud.ibm.com/docs/vpc?topic=vpc-file-storage-vpc-about