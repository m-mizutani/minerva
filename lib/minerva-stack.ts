import * as cdk from "@aws-cdk/core";
import * as lambda from "@aws-cdk/aws-lambda";
import * as s3 from "@aws-cdk/aws-s3";
import * as sns from "@aws-cdk/aws-sns";
import * as iam from "@aws-cdk/aws-iam";
import * as dynamodb from "@aws-cdk/aws-dynamodb";
import * as sqs from "@aws-cdk/aws-sqs";
import * as glue from "@aws-cdk/aws-glue";
import * as subscriptions from "@aws-cdk/aws-sns-subscriptions";
import * as apigateway from "@aws-cdk/aws-apigateway";

import {
  DynamoEventSource,
  SqsEventSource,
} from "@aws-cdk/aws-lambda-event-sources";

interface MinervaArguments {
  lambdaRole: iam.IRole;
  dataS3Bucket: s3.IBucket;
  dataS3Prefix: string;
  dataSNSTopic: sns.ITopic;
  athenaDatabaseName: string;
  sentryDSN?: string;
  sentryEnv?: string;
  logLevel?: string;
  concurrentExecution?: number;
}

export class MinervaStack extends cdk.Stack {
  constructor(
    scope: cdk.Construct,
    id: string,
    args: MinervaArguments,
    props?: cdk.StackProps
  ) {
    super(scope, id, props);

    const buildPath = lambda.Code.asset("./build");
    const indexTableName = "indices";
    const objectTableName = "objects";
    const messageTableName = "messages";

    // DynamoDB
    const metaTable = new dynamodb.Table(this, "metaTable", {
      partitionKey: { name: "pk", type: dynamodb.AttributeType.STRING },
      timeToLiveAttribute: "expires_at",
      readCapacity: 20,
      writeCapacity: 20,
    });

    const searchTable = new dynamodb.Table(this, "searchTable", {
      partitionKey: { name: "pk", type: dynamodb.AttributeType.STRING },
      sortKey: { name: "sk", type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      timeToLiveAttribute: "expires_at",
    });

    // SQS
    const indexerQueue = new sqs.Queue(this, "indexerQueue", {
      visibilityTimeout: cdk.Duration.seconds(600),
    });
    args.dataSNSTopic.addSubscription(
      new subscriptions.SqsSubscription(indexerQueue)
    );

    const mergeQueue = new sqs.Queue(this, "mergeQueue", {
      visibilityTimeout: cdk.Duration.seconds(450),
    });
    const partitionQueue = new sqs.Queue(this, "partitionQueue");

    const indexerQueuePolicy = new sqs.QueuePolicy(this, "indexerQueuePolicy", {
      queues: [indexerQueue],
    });

    // Lambda Functions
    const indexer = new lambda.Function(this, "indexer", {
      runtime: lambda.Runtime.GO_1_X,
      handler: "indexer",
      code: buildPath,
      role: args.lambdaRole,
      timeout: cdk.Duration.seconds(600),
      memorySize: 2048,
      environment: {
        S3_REGION: args.dataS3Bucket.bucketRegionalDomainName,
        S3_BUCKET: args.dataS3Bucket.bucketName,
        S3_PREFIX: args.dataS3Prefix,
        INDEX_TABLE_NAME: indexTableName,
        MESSAGE_TABLE_NAME: messageTableName,
        META_TABLE_NAME: metaTable.tableName,
        PARTITION_QUEUE: partitionQueue.queueUrl,
      },
      reservedConcurrentExecutions: args.concurrentExecution,
      events: [new SqsEventSource(indexerQueue, { batchSize: 1 })],
    });

    const listIndexObject = new lambda.Function(this, "listIndexObject", {
      runtime: lambda.Runtime.GO_1_X,
      handler: "listIndexObject",
      code: buildPath,
      role: args.lambdaRole,
      timeout: cdk.Duration.seconds(300),
      memorySize: 1024,
      environment: {
        S3_REGION: args.dataS3Bucket.bucketRegionalDomainName,
        S3_BUCKET: args.dataS3Bucket.bucketName,
        S3_PREFIX: args.dataS3Prefix,
        MERGE_QUEUE: mergeQueue.queueUrl,
      },
      reservedConcurrentExecutions: args.concurrentExecution,
      // events: , TBD
    });

    const mergeIndexObject = new lambda.Function(this, "mergeIndexObject", {
      runtime: lambda.Runtime.GO_1_X,
      handler: "mergeIndexObject",
      code: buildPath,
      role: args.lambdaRole,
      timeout: cdk.Duration.seconds(450),
      memorySize: 2048,
      reservedConcurrentExecutions: args.concurrentExecution,
      events: [new SqsEventSource(mergeQueue, { batchSize: 1 })],
    });

    const makePartition = new lambda.Function(this, "makePartition", {
      runtime: lambda.Runtime.GO_1_X,
      handler: "makePartition",
      code: buildPath,
      role: args.lambdaRole,
      timeout: cdk.Duration.seconds(450),
      memorySize: 2048,
      environment: {
        ATHENA_DB_NAME: args.athenaDatabaseName,
        OBJECT_TABLE_NAME: indexTableName,
        META_TABLE_NAME: metaTable.tableName,
        S3_BUCKET: args.dataS3Bucket.bucketName,
        S3_PREFIX: args.dataS3Prefix,
      },
      reservedConcurrentExecutions: args.concurrentExecution,
      events: [new SqsEventSource(partitionQueue, { batchSize: 1 })],
    });

    const indexDB = new glue.Database(this, "indexDB", {
      databaseName: args.athenaDatabaseName,
    });

    const indexTable = new glue.Table(this, "indexTable", {
      tableName: indexTableName,
      database: indexDB,
      partitionKeys: [{ name: "dt", type: glue.Schema.STRING }],
      columns: [
        { name: "tag", type: glue.Schema.STRING },
        { name: "timestamp", type: glue.Schema.BIG_INT },
        { name: "field", type: glue.Schema.STRING },
        { name: "term", type: glue.Schema.STRING },
        { name: "object_id", type: glue.Schema.BIG_INT },
        { name: "seq", type: glue.Schema.INTEGER },
      ],
      bucket: args.dataS3Bucket,
      s3Prefix: args.dataS3Prefix + "indices/",
      dataFormat: glue.DataFormat.PARQUET,
    });

    const messageTable = new glue.Table(this, "messageTable", {
      tableName: messageTableName,
      database: indexDB,
      partitionKeys: [{ name: "dt", type: glue.Schema.STRING }],
      columns: [
        { name: "timestamp", type: glue.Schema.BIG_INT },
        { name: "object_id", type: glue.Schema.BIG_INT },
        { name: "seq", type: glue.Schema.INTEGER },
        { name: "message", type: glue.Schema.STRING },
      ],
      bucket: args.dataS3Bucket,
      s3Prefix: args.dataS3Prefix + "messages/",
      dataFormat: glue.DataFormat.PARQUET,
    });

    // API handler
    const apiHandler = new lambda.Function(this, "apiHandler", {
      runtime: lambda.Runtime.GO_1_X,
      handler: "apiHandler",
      code: buildPath,
      role: args.lambdaRole,
      timeout: cdk.Duration.seconds(120),
      memorySize: 2048,
      environment: {
        S3_REGION: args.dataS3Bucket.bucketRegionalDomainName,
        S3_BUCKET: args.dataS3Bucket.bucketName,
        S3_PREFIX: args.dataS3Prefix,
        ATHENA_DB_NAME: indexDB.databaseName,
        INDEX_TABLE_NAME: indexTableName,
        MESSAGE_TABLE_NAME: messageTableName,
        META_TABLE_NAME: searchTable.tableName,
      },
    });

    const api = new apigateway.LambdaRestApi(this, "minervaAPI", {
      handler: apiHandler,
      proxy: false,
      cloudWatchRole: false,
      endpointTypes: [apigateway.EndpointType.PRIVATE],
      policy: new iam.PolicyDocument({
        statements: [
          new iam.PolicyStatement({
            actions: ["execute-api:Invoke"],
            resources: ["execute-api:/*/*"],
            effect: iam.Effect.ALLOW,
            principals: [new iam.AnyPrincipal()],
          }),
        ],
      }),
    });

    const v1 = api.root.addResource("api").addResource("v1");
    const searchAPI = v1.addResource("search");
    searchAPI.addMethod("POST");
    searchAPI.addResource("{search_id}").addMethod("GET");
    searchAPI.addResource("{search_id}").addResource("logs").addMethod("GET");
    searchAPI
      .addResource("{search_id}")
      .addResource("timeseries")
      .addMethod("GET");
  }
}
