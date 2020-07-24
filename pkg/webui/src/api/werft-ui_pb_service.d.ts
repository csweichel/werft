// package: v1
// file: werft-ui.proto

import * as werft_ui_pb from "./werft-ui_pb";
import {grpc} from "@improbable-eng/grpc-web";

type WerftUIListJobSpecs = {
  readonly methodName: string;
  readonly service: typeof WerftUI;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof werft_ui_pb.ListJobSpecsRequest;
  readonly responseType: typeof werft_ui_pb.ListJobSpecsResponse;
};

type WerftUIIsReadOnly = {
  readonly methodName: string;
  readonly service: typeof WerftUI;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof werft_ui_pb.IsReadOnlyRequest;
  readonly responseType: typeof werft_ui_pb.IsReadOnlyResponse;
};

export class WerftUI {
  static readonly serviceName: string;
  static readonly ListJobSpecs: WerftUIListJobSpecs;
  static readonly IsReadOnly: WerftUIIsReadOnly;
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

export class WerftUIClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  listJobSpecs(requestMessage: werft_ui_pb.ListJobSpecsRequest, metadata?: grpc.Metadata): ResponseStream<werft_ui_pb.ListJobSpecsResponse>;
  isReadOnly(
    requestMessage: werft_ui_pb.IsReadOnlyRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: werft_ui_pb.IsReadOnlyResponse|null) => void
  ): UnaryResponse;
  isReadOnly(
    requestMessage: werft_ui_pb.IsReadOnlyRequest,
    callback: (error: ServiceError|null, responseMessage: werft_ui_pb.IsReadOnlyResponse|null) => void
  ): UnaryResponse;
}

