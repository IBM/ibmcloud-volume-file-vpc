## How to execute E2E?

1. Create a VPC Cluster
2. Export the KUBECONFIG
   In kube config file use absolute path for `certificate-authority`, `client-certificate` and `client-key`
3. Deploy the Driver (with SC)
4. Export environment variables
   ```
   # Mandatory
   export GO111MODULE=on
   export GOPATH=<GOPATH>
   export KUBECONFIG=<absolute-path-to-kubeconfig>
   export E2E_TEST_RESULT=<absolute-path to a file where the results should be redirected>
   export TEST_ENV=<stage/prod>
   export IC_REGION=<us-south>
   export IC_API_KEY_PROD=<prod API key> | export IC_API_KEY_STAG=<stage API key>
   export e2e_addon_version=<1.2 or 2.0>
   export icrImage=<Give the image which will be used by pods>
   export SC=<storage-class-name-with-delete-reclaim-policy>
   export SC_RETAIN=<storage-class-name-with-retain-reclaim-policy>

   # Optional
   export E2E_POD_COUNT="1"
   export E2E_PVC_COUNT="1"
   ```

5. Test DP2 profile with deployment
   ```
   ginkgo -v -nodes=1 --focus="\[ics-e2e\] \[sc\] \[with-deploy\]" ./ginkgo_tests -- --kubeconfig=$KUBECONFIG -e2e-verify-service-account=false
   ```
6. Test volume expansion
   ```
   ginkgo -v -nodes=1 --focus="\[ics-e2e\] \[resize\] \[pv\]" ./ginkgo_tests -- --kubeconfig=$KUBECONFIG -e2e-verify-service-account=false
   ```
7. Test EIT enabled volume test cases
   ```
   ginkgo -v -nodes=1 --focus="\[ics-e2e\] \[eit\]" ./ginkgo_tests -- --kubeconfig=$KUBECONFIG -e2e-verify-service-account=false
   ```
   
8. Test RFS profile and it's storage classes
   ```
   ginkgo -v -nodes=1 --focus="\[ics-e2e\] \[sc_rfs\]" ./ginkgo_tests -- --kubeconfig=$KUBECONFIG -e2e-verify-service-account=false
   ```

9. Test Snapshot for DP2 and RFS profile 
   ```
   ginkgo -v -nodes=1 --focus="\[ics-e2e\] \[snapshot\]" ./ginkgo_tests -- --kubeconfig=$KUBECONFIG -e2e-verify-service-account=false
   ```

10. Test Capacity Roundoff for DP2 profile
    ```
    ginkgo -v -nodes=1 --focus="\[ics-e2e\] \[roundoff\]" ./ginkgo_tests -- --kubeconfig=$KUBECONFIG -e2e-verify-service-account=false
    ```
