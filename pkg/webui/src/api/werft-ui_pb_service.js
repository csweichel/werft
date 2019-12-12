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

exports.WerftUIClient = WerftUIClient;

