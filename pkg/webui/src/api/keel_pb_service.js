// package: v1
// file: keel.proto

var keel_pb = require("./keel_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var KeelService = (function () {
  function KeelService() {}
  KeelService.serviceName = "v1.KeelService";
  return KeelService;
}());

KeelService.ListJobs = {
  methodName: "ListJobs",
  service: KeelService,
  requestStream: false,
  responseStream: false,
  requestType: keel_pb.ListJobsRequest,
  responseType: keel_pb.ListJobsResponse
};

KeelService.Listen = {
  methodName: "Listen",
  service: KeelService,
  requestStream: false,
  responseStream: true,
  requestType: keel_pb.ListenRequest,
  responseType: keel_pb.ListenResponse
};

exports.KeelService = KeelService;

function KeelServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

KeelServiceClient.prototype.listJobs = function listJobs(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(KeelService.ListJobs, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

KeelServiceClient.prototype.listen = function listen(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(KeelService.Listen, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onMessage: function (responseMessage) {
      listeners.data.forEach(function (handler) {
        handler(responseMessage);
      });
    },
    onEnd: function (status, statusMessage, trailers) {
      listeners.status.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners.end.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners = null;
    }
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

exports.KeelServiceClient = KeelServiceClient;

