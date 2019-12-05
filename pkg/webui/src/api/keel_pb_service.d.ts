// package: v1
// file: keel.proto

import * as keel_pb from "./keel_pb";
import {grpc} from "@improbable-eng/grpc-web";

type KeelServiceStartLocalJob = {
  readonly methodName: string;
  readonly service: typeof KeelService;
  readonly requestStream: true;
  readonly responseStream: false;
  readonly requestType: typeof keel_pb.StartLocalJobRequest;
  readonly responseType: typeof keel_pb.StartJobResponse;
};

type KeelServiceListJobs = {
  readonly methodName: string;
  readonly service: typeof KeelService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof keel_pb.ListJobsRequest;
  readonly responseType: typeof keel_pb.ListJobsResponse;
};

type KeelServiceSubscribe = {
  readonly methodName: string;
  readonly service: typeof KeelService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof keel_pb.SubscribeRequest;
  readonly responseType: typeof keel_pb.SubscribeResponse;
};

type KeelServiceListen = {
  readonly methodName: string;
  readonly service: typeof KeelService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof keel_pb.ListenRequest;
  readonly responseType: typeof keel_pb.ListenResponse;
};

export class KeelService {
  static readonly serviceName: string;
  static readonly StartLocalJob: KeelServiceStartLocalJob;
  static readonly ListJobs: KeelServiceListJobs;
  static readonly Subscribe: KeelServiceSubscribe;
  static readonly Listen: KeelServiceListen;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(type: 'data', handler: (message: ResT) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'end', handler: (status?: Status) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'status', handler: (status: Status) => void): BidirectionalStream<ReqT, ResT>;
}

export class KeelServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  startLocalJob(metadata?: grpc.Metadata): RequestStream<keel_pb.StartLocalJobRequest>;
  listJobs(
    requestMessage: keel_pb.ListJobsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: keel_pb.ListJobsResponse|null) => void
  ): UnaryResponse;
  listJobs(
    requestMessage: keel_pb.ListJobsRequest,
    callback: (error: ServiceError|null, responseMessage: keel_pb.ListJobsResponse|null) => void
  ): UnaryResponse;
  subscribe(requestMessage: keel_pb.SubscribeRequest, metadata?: grpc.Metadata): ResponseStream<keel_pb.SubscribeResponse>;
  listen(requestMessage: keel_pb.ListenRequest, metadata?: grpc.Metadata): ResponseStream<keel_pb.ListenResponse>;
}

