// package: v1
// file: keel.proto

import * as jspb from "google-protobuf";

export class ListJobsRequest extends jspb.Message {
  clearFilterList(): void;
  getFilterList(): Array<AnnotationFilter>;
  setFilterList(value: Array<AnnotationFilter>): void;
  addFilter(value?: AnnotationFilter, index?: number): AnnotationFilter;

  getRunningOnly(): boolean;
  setRunningOnly(value: boolean): void;

  getStart(): number;
  setStart(value: number): void;

  getLimit(): number;
  setLimit(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListJobsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListJobsRequest): ListJobsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListJobsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListJobsRequest;
  static deserializeBinaryFromReader(message: ListJobsRequest, reader: jspb.BinaryReader): ListJobsRequest;
}

export namespace ListJobsRequest {
  export type AsObject = {
    filterList: Array<AnnotationFilter.AsObject>,
    runningOnly: boolean,
    start: number,
    limit: number,
  }
}

export class AnnotationFilter extends jspb.Message {
  clearTermsList(): void;
  getTermsList(): Array<AnnotationFilterTerm>;
  setTermsList(value: Array<AnnotationFilterTerm>): void;
  addTerms(value?: AnnotationFilterTerm, index?: number): AnnotationFilterTerm;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AnnotationFilter.AsObject;
  static toObject(includeInstance: boolean, msg: AnnotationFilter): AnnotationFilter.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AnnotationFilter, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AnnotationFilter;
  static deserializeBinaryFromReader(message: AnnotationFilter, reader: jspb.BinaryReader): AnnotationFilter;
}

export namespace AnnotationFilter {
  export type AsObject = {
    termsList: Array<AnnotationFilterTerm.AsObject>,
  }
}

export class AnnotationFilterTerm extends jspb.Message {
  getAnnotation(): string;
  setAnnotation(value: string): void;

  getValue(): string;
  setValue(value: string): void;

  getOperation(): AnnotationFilterOpMap[keyof AnnotationFilterOpMap];
  setOperation(value: AnnotationFilterOpMap[keyof AnnotationFilterOpMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AnnotationFilterTerm.AsObject;
  static toObject(includeInstance: boolean, msg: AnnotationFilterTerm): AnnotationFilterTerm.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AnnotationFilterTerm, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AnnotationFilterTerm;
  static deserializeBinaryFromReader(message: AnnotationFilterTerm, reader: jspb.BinaryReader): AnnotationFilterTerm;
}

export namespace AnnotationFilterTerm {
  export type AsObject = {
    annotation: string,
    value: string,
    operation: AnnotationFilterOpMap[keyof AnnotationFilterOpMap],
  }
}

export class ListJobsResponse extends jspb.Message {
  getTotal(): number;
  setTotal(value: number): void;

  clearResultList(): void;
  getResultList(): Array<JobStatus>;
  setResultList(value: Array<JobStatus>): void;
  addResult(value?: JobStatus, index?: number): JobStatus;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListJobsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListJobsResponse): ListJobsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListJobsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListJobsResponse;
  static deserializeBinaryFromReader(message: ListJobsResponse, reader: jspb.BinaryReader): ListJobsResponse;
}

export namespace ListJobsResponse {
  export type AsObject = {
    total: number,
    resultList: Array<JobStatus.AsObject>,
  }
}

export class ListenRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getUpdates(): boolean;
  setUpdates(value: boolean): void;

  getLogs(): boolean;
  setLogs(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListenRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListenRequest): ListenRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListenRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListenRequest;
  static deserializeBinaryFromReader(message: ListenRequest, reader: jspb.BinaryReader): ListenRequest;
}

export namespace ListenRequest {
  export type AsObject = {
    name: string,
    updates: boolean,
    logs: boolean,
  }
}

export class ListenResponse extends jspb.Message {
  hasUpdate(): boolean;
  clearUpdate(): void;
  getUpdate(): JobStatus | undefined;
  setUpdate(value?: JobStatus): void;

  hasSlice(): boolean;
  clearSlice(): void;
  getSlice(): LogSliceEvent | undefined;
  setSlice(value?: LogSliceEvent): void;

  getContentCase(): ListenResponse.ContentCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListenResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListenResponse): ListenResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListenResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListenResponse;
  static deserializeBinaryFromReader(message: ListenResponse, reader: jspb.BinaryReader): ListenResponse;
}

export namespace ListenResponse {
  export type AsObject = {
    update?: JobStatus.AsObject,
    slice?: LogSliceEvent.AsObject,
  }

  export enum ContentCase {
    CONTENT_NOT_SET = 0,
    UPDATE = 1,
    SLICE = 2,
  }
}

export class JobStatus extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasMetadata(): boolean;
  clearMetadata(): void;
  getMetadata(): JobMetadata | undefined;
  setMetadata(value?: JobMetadata): void;

  getPhase(): JobPhaseMap[keyof JobPhaseMap];
  setPhase(value: JobPhaseMap[keyof JobPhaseMap]): void;

  hasConditions(): boolean;
  clearConditions(): void;
  getConditions(): JobConditions | undefined;
  setConditions(value?: JobConditions): void;

  getDetails(): string;
  setDetails(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobStatus.AsObject;
  static toObject(includeInstance: boolean, msg: JobStatus): JobStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobStatus;
  static deserializeBinaryFromReader(message: JobStatus, reader: jspb.BinaryReader): JobStatus;
}

export namespace JobStatus {
  export type AsObject = {
    name: string,
    metadata?: JobMetadata.AsObject,
    phase: JobPhaseMap[keyof JobPhaseMap],
    conditions?: JobConditions.AsObject,
    details: string,
  }
}

export class JobMetadata extends jspb.Message {
  clearAnnotationsList(): void;
  getAnnotationsList(): Array<Annotation>;
  setAnnotationsList(value: Array<Annotation>): void;
  addAnnotations(value?: Annotation, index?: number): Annotation;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: JobMetadata): JobMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobMetadata;
  static deserializeBinaryFromReader(message: JobMetadata, reader: jspb.BinaryReader): JobMetadata;
}

export namespace JobMetadata {
  export type AsObject = {
    annotationsList: Array<Annotation.AsObject>,
  }
}

export class Annotation extends jspb.Message {
  getKey(): string;
  setKey(value: string): void;

  getValue(): string;
  setValue(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Annotation.AsObject;
  static toObject(includeInstance: boolean, msg: Annotation): Annotation.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Annotation, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Annotation;
  static deserializeBinaryFromReader(message: Annotation, reader: jspb.BinaryReader): Annotation;
}

export namespace Annotation {
  export type AsObject = {
    key: string,
    value: string,
  }
}

export class JobConditions extends jspb.Message {
  getSuccess(): boolean;
  setSuccess(value: boolean): void;

  getFailureCount(): number;
  setFailureCount(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JobConditions.AsObject;
  static toObject(includeInstance: boolean, msg: JobConditions): JobConditions.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JobConditions, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JobConditions;
  static deserializeBinaryFromReader(message: JobConditions, reader: jspb.BinaryReader): JobConditions;
}

export namespace JobConditions {
  export type AsObject = {
    success: boolean,
    failureCount: number,
  }
}

export class LogSliceEvent extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPhase(): LogSlicePhaseMap[keyof LogSlicePhaseMap];
  setPhase(value: LogSlicePhaseMap[keyof LogSlicePhaseMap]): void;

  getPayload(): string;
  setPayload(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LogSliceEvent.AsObject;
  static toObject(includeInstance: boolean, msg: LogSliceEvent): LogSliceEvent.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LogSliceEvent, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LogSliceEvent;
  static deserializeBinaryFromReader(message: LogSliceEvent, reader: jspb.BinaryReader): LogSliceEvent;
}

export namespace LogSliceEvent {
  export type AsObject = {
    name: string,
    phase: LogSlicePhaseMap[keyof LogSlicePhaseMap],
    payload: string,
  }
}

export interface AnnotationFilterOpMap {
  OP_EQUALS: 0;
  OP_STARTS_WITH: 1;
  OP_ENDS_WITH: 2;
  OP_CONTAINS: 3;
  OP_HAS_KEY: 4;
}

export const AnnotationFilterOp: AnnotationFilterOpMap;

export interface JobPhaseMap {
  PHASE_UNKNOWN: 0;
  PHASE_PREPARING: 1;
  PHASE_STARTING: 2;
  PHASE_RUNNING: 3;
  PHASE_DONE: 4;
  PHASE_CLEANUP: 5;
}

export const JobPhase: JobPhaseMap;

export interface LogSlicePhaseMap {
  SLICE_ABANDONED: 0;
  SLICE_CHECKPOINT: 1;
  SLICE_START: 2;
  SLICE_CONTENT: 3;
  SLICE_END: 4;
}

export const LogSlicePhase: LogSlicePhaseMap;

