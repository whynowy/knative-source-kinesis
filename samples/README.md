# Kinesis Event Source Sample

## Deployment Steps

### Prerequisites

1.  Create an [AWS Kinesis Stream](https://aws.amazon.com/kinesis/).

1.  Setup
    [Knative Serving](https://github.com/knative/docs/tree/master/serving).

1.  Setup
    [Knative Eventing](https://github.com/knative/docs/tree/master/eventing).

1.  Install the
    [in-memory `ClusterChannelProvisioner`](https://github.com/knative/eventing/tree/master/config/provisioners/in-memory-channel).

    - Note that you can skip this if it is already in place, or you choose to
      use a different type of `Channel`. If so, you will need to modify
      `channel.yaml` before deploying it.

1.  [__Skip this step if using KIAM approach__] Acquire
    [AWS Credentials](https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html)
    for the same account. Your credentials file should look like this:

        [default]
        aws_access_key_id = ...
        aws_secret_access_key = ...

         1. Create a secret for the downloaded key:

             ```shell
             kubectl -n knative-sources create secret generic kinesis-source-credentials --from-file=credentials=PATH_TO_CREDENTIALS_FILE
             ```

1.  [__Skip this step if using Credentials approach__] Create a
    [cross account IAM role](https://docs.aws.amazon.com/IAM/latest/UserGuide/tutorial_cross-account-with-roles.html)
    and set the policy to be able to access Kiness stream, and set it to trust
    the IAM role you acquired for the applications running in your namespace
    (refer to [KIAM](https://github.com/uswitch/kiam)).

### Deployment

1.  Deploy the `KinesisSource` controller as part of Knative installation.

    ```shell
    ko apply -f config/
    ```

1.  Create a `Channel`. You can use your own `Channel` or use the provided
    sample, which creates `cj-3`. If you use your own `Channel` with a different
    name, then you will need to alter other commands later.

    ```shell
    kubectl -n default apply -f samples/channel.yaml
    ```

1.  Configure source in `samples/source-cred.yaml` [`credentials` approach], or
    `samples/source-kiam.yaml` [`KIAM` approach].

    - `streamName` should be replaced with your Kinesis stream name.

    - `region` region of your Kinesis stream.

    - `awsCredsSecret` [`credentials` approach] should be replaced with the name
      of the k8s secret that contains the AWS credentials.

    - `kiamOptions` [`KIAM` approach] set proper values for `assignedIamRole`
      and `kinesisIamRoleArn` as commented.

    - `cj-3` should be replaced with the name of the `Channel` you want messages
      sent to. If you deployed an unaltered `channel.yaml` then you can leave it
      as `cj-3`.

### Subscriber

In order to check the `KinesisSource` is fully working, we will create a simple
Knative Service that dumps incoming messages to its log and create a
`Subscription` from the `Channel` to that Knative Service.

1. Deploy `message-dumper` Knativer service.

   ```shell
   kubectl -n default apply -f samples/service.yaml
   ```

1. If the deployed `KinesisSource` is pointing at a `Channel` other than `cj-3`,
   modify `sub.yaml` by replacing `cj-3` with that `Channel`'s name.

1. Deploy `subscriber.yaml`.

   ```shell
   kubectl -n default apply -f samples/sub.yaml
   ```

### Publish

Publish messages to your Kinesis stream.

**TBA**

### Verify

We will verify that the message was sent to the Knative eventing system by
looking at message dumper logs.

```shell
kubectl -n default logs -l serving.knative.dev/service=message-dumper -c user-container --since=10m
```

You should see log lines showing the request headers and body from the source:

**TBA**
