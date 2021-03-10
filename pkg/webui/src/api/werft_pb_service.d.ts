// package: v1
// file: werft.proto

import * as werft_pb from "./werft_pb";
import {grpc} from "@improbable-eng/grpc-web";

type WerftServiceStartLocalJob = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: true;
  readonly responseStream: false;
  readonly requestType: typeof werft_pb.StartLocalJobRequest;
  readonly responseType: typeof werft_pb.StartJobResponse;
};

type WerftServiceStartGitHubJob = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof werft_pb.StartGitHubJobRequest;
  readonly responseType: typeof werft_pb.StartJobResponse;
};

type WerftServiceStartFromPreviousJob = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof werft_pb.StartFromPreviousJobRequest;
  readonly responseType: typeof werft_pb.StartJobResponse;
};

type WerftServiceStartJob = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof werft_pb.StartJobRequest;
  readonly responseType: typeof werft_pb.StartJobResponse;
};

type WerftServiceListJobs = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof werft_pb.ListJobsRequest;
  readonly responseType: typeof werft_pb.ListJobsResponse;
};

type WerftServiceSubscribe = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof werft_pb.SubscribeRequest;
  readonly responseType: typeof werft_pb.SubscribeResponse;
};

type WerftServiceGetJob = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof werft_pb.GetJobRequest;
  readonly responseType: typeof werft_pb.GetJobResponse;
};

type WerftServiceListen = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof werft_pb.ListenRequest;
  readonly responseType: typeof werft_pb.ListenResponse;
};

type WerftServiceStopJob = {
  readonly methodName: string;
  readonly service: typeof WerftService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof werft_pb.StopJobRequest;
  readonly responseType: typeof werft_pb.StopJobResponse;
};

export class WerftService {
  static readonly serviceName: string;
  static readonly StartLocalJob: WerftServiceStartLocalJob;
  static readonly StartGitHubJob: WerftServiceStartGitHubJob;
  static readonly StartFromPreviousJob: WerftServiceStartFromPreviousJob;
  static readonly StartJob: WerftServiceStartJob;
  static readonly ListJobs: WerftServiceListJobs;
  static readonly Subscribe: WerftServiceSubscribe;
  static readonly GetJob: WerftServiceGetJob;
  static readonly Listen: WerftServiceListen;
  static readonly StopJob: WerftServiceStopJob;
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

export class WerftServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  startLocalJob(metadata?: grpc.Metadata): RequestStream<werft_pb.StartLocalJobRequest>;
  startGitHubJob(
    requestMessage: werft_pb.StartGitHubJobRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StartJobResponse|null) => void
  ): UnaryResponse;
  startGitHubJob(
    requestMessage: werft_pb.StartGitHubJobRequest,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StartJobResponse|null) => void
  ): UnaryResponse;
  startFromPreviousJob(
    requestMessage: werft_pb.StartFromPreviousJobRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StartJobResponse|null) => void
  ): UnaryResponse;
  startFromPreviousJob(
    requestMessage: werft_pb.StartFromPreviousJobRequest,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StartJobResponse|null) => void
  ): UnaryResponse;
  startJob(
    requestMessage: werft_pb.StartJobRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StartJobResponse|null) => void
  ): UnaryResponse;
  startJob(
    requestMessage: werft_pb.StartJobRequest,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StartJobResponse|null) => void
  ): UnaryResponse;
  listJobs(
    requestMessage: werft_pb.ListJobsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: werft_pb.ListJobsResponse|null) => void
  ): UnaryResponse;
  listJobs(
    requestMessage: werft_pb.ListJobsRequest,
    callback: (error: ServiceError|null, responseMessage: werft_pb.ListJobsResponse|null) => void
  ): UnaryResponse;
  subscribe(requestMessage: werft_pb.SubscribeRequest, metadata?: grpc.Metadata): ResponseStream<werft_pb.SubscribeResponse>;
  getJob(
    requestMessage: werft_pb.GetJobRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: werft_pb.GetJobResponse|null) => void
  ): UnaryResponse;
  getJob(
    requestMessage: werft_pb.GetJobRequest,
    callback: (error: ServiceError|null, responseMessage: werft_pb.GetJobResponse|null) => void
  ): UnaryResponse;
  listen(requestMessage: werft_pb.ListenRequest, metadata?: grpc.Metadata): ResponseStream<werft_pb.ListenResponse>;
  stopJob(
    requestMessage: werft_pb.StopJobRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StopJobResponse|null) => void
  ): UnaryResponse;
  stopJob(
    requestMessage: werft_pb.StopJobRequest,
    callback: (error: ServiceError|null, responseMessage: werft_pb.StopJobResponse|null) => void
  ): UnaryResponse;
}

