// package: v1
// file: werft.proto

var werft_pb = require("./werft_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var WerftService = (function () {
  function WerftService() {}
  WerftService.serviceName = "v1.WerftService";
  return WerftService;
}());

WerftService.StartLocalJob = {
  methodName: "StartLocalJob",
  service: WerftService,
  requestStream: true,
  responseStream: false,
  requestType: werft_pb.StartLocalJobRequest,
  responseType: werft_pb.StartJobResponse
};

WerftService.StartGitHubJob = {
  methodName: "StartGitHubJob",
  service: WerftService,
  requestStream: false,
  responseStream: false,
  requestType: werft_pb.StartGitHubJobRequest,
  responseType: werft_pb.StartJobResponse
};

WerftService.StartFromPreviousJob = {
  methodName: "StartFromPreviousJob",
  service: WerftService,
  requestStream: false,
  responseStream: false,
  requestType: werft_pb.StartFromPreviousJobRequest,
  responseType: werft_pb.StartJobResponse
};

WerftService.StartJob = {
  methodName: "StartJob",
  service: WerftService,
  requestStream: false,
  responseStream: false,
  requestType: werft_pb.StartJobRequest,
  responseType: werft_pb.StartJobResponse
};

WerftService.ListJobs = {
  methodName: "ListJobs",
  service: WerftService,
  requestStream: false,
  responseStream: false,
  requestType: werft_pb.ListJobsRequest,
  responseType: werft_pb.ListJobsResponse
};

WerftService.Subscribe = {
  methodName: "Subscribe",
  service: WerftService,
  requestStream: false,
  responseStream: true,
  requestType: werft_pb.SubscribeRequest,
  responseType: werft_pb.SubscribeResponse
};

WerftService.GetJob = {
  methodName: "GetJob",
  service: WerftService,
  requestStream: false,
  responseStream: false,
  requestType: werft_pb.GetJobRequest,
  responseType: werft_pb.GetJobResponse
};

WerftService.Listen = {
  methodName: "Listen",
  service: WerftService,
  requestStream: false,
  responseStream: true,
  requestType: werft_pb.ListenRequest,
  responseType: werft_pb.ListenResponse
};

WerftService.StopJob = {
  methodName: "StopJob",
  service: WerftService,
  requestStream: false,
  responseStream: false,
  requestType: werft_pb.StopJobRequest,
  responseType: werft_pb.StopJobResponse
};

exports.WerftService = WerftService;

function WerftServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

WerftServiceClient.prototype.startLocalJob = function startLocalJob(metadata) {
  var listeners = {
    end: [],
    status: []
  };
  var client = grpc.client(WerftService.StartLocalJob, {
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport
  });
  client.onEnd(function (status, statusMessage, trailers) {
    listeners.status.forEach(function (handler) {
      handler({ code: status, details: statusMessage, metadata: trailers });
    });
    listeners.end.forEach(function (handler) {
      handler({ code: status, details: statusMessage, metadata: trailers });
    });
    listeners = null;
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    write: function (requestMessage) {
      if (!client.started) {
        client.start(metadata);
      }
      client.send(requestMessage);
      return this;
    },
    end: function () {
      client.finishSend();
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

WerftServiceClient.prototype.startGitHubJob = function startGitHubJob(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(WerftService.StartGitHubJob, {
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

WerftServiceClient.prototype.startFromPreviousJob = function startFromPreviousJob(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(WerftService.StartFromPreviousJob, {
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

WerftServiceClient.prototype.startJob = function startJob(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(WerftService.StartJob, {
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

WerftServiceClient.prototype.listJobs = function listJobs(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(WerftService.ListJobs, {
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

WerftServiceClient.prototype.subscribe = function subscribe(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(WerftService.Subscribe, {
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

WerftServiceClient.prototype.getJob = function getJob(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(WerftService.GetJob, {
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

WerftServiceClient.prototype.listen = function listen(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(WerftService.Listen, {
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

WerftServiceClient.prototype.stopJob = function stopJob(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(WerftService.StopJob, {
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

exports.WerftServiceClient = WerftServiceClient;

