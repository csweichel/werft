// package: v1
// file: werft-ui.proto

var werft_ui_pb = require("./werft-ui_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var WerftUI = (function () {
  function WerftUI() {}
  WerftUI.serviceName = "v1.WerftUI";
  return WerftUI;
}());

WerftUI.ListJobSpecs = {
  methodName: "ListJobSpecs",
  service: WerftUI,
  requestStream: false,
  responseStream: true,
  requestType: werft_ui_pb.ListJobSpecsRequest,
  responseType: werft_ui_pb.ListJobSpecsResponse
};

WerftUI.IsReadOnly = {
  methodName: "IsReadOnly",
  service: WerftUI,
  requestStream: false,
  responseStream: false,
  requestType: werft_ui_pb.IsReadOnlyRequest,
  responseType: werft_ui_pb.IsReadOnlyResponse
};

exports.WerftUI = WerftUI;

function WerftUIClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

WerftUIClient.prototype.listJobSpecs = function listJobSpecs(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(WerftUI.ListJobSpecs, {
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

WerftUIClient.prototype.isReadOnly = function isReadOnly(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(WerftUI.IsReadOnly, {
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

exports.WerftUIClient = WerftUIClient;

