// package: v1
// file: keel.proto

import * as keel_pb from "./keel_pb";
import {grpc} from "@improbable-eng/grpc-web";

type KeelServiceListJobs = {
  readonly methodName: string;
  readonly service: typeof KeelService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof keel_pb.ListJobsRequest;
  readonly responseType: typeof keel_pb.ListJobsResponse;
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
  static readonly ListJobs: KeelServiceListJobs;
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
  listJobs(
    requestMessage: keel_pb.ListJobsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: keel_pb.ListJobsResponse|null) => void
  ): UnaryResponse;
  listJobs(
    requestMessage: keel_pb.ListJobsRequest,
    callback: (error: ServiceError|null, responseMessage: keel_pb.ListJobsResponse|null) => void
  ): UnaryResponse;
  listen(requestMessage: keel_pb.ListenRequest, metadata?: grpc.Metadata): ResponseStream<keel_pb.ListenResponse>;
}

