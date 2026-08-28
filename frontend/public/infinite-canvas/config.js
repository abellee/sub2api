// Runtime configuration regenerated from environment variables by docker-entrypoint.sh at container startup.
// Local development and deployments without the entrypoint use these empty defaults, leaving analytics disabled.
window.__RUNTIME_CONFIG__ = window.__RUNTIME_CONFIG__ || {};

// Keep local development same-origin so the embedded agent does not hit CORS.
(function forceLocalCanvasProxy() {
  var isLocalHost = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1';
  if (!isLocalHost) return;

  var upstreamOrigin = 'https://api.llmfree.work';
  var localPrefix = '/infinite-api';
  var rewriteApiUrl = function (value) {
    if (typeof value !== 'string' || value.indexOf(upstreamOrigin) !== 0) return value;
    return localPrefix + value.slice(upstreamOrigin.length);
  };

  var nativeFetch = window.fetch;
  window.fetch = function (input, init) {
    if (typeof input === 'string') {
      input = rewriteApiUrl(input);
    } else if (input && typeof input.url === 'string') {
      input = new Request(rewriteApiUrl(input.url), input);
    }
    return nativeFetch.call(this, input, init);
  };

  var nativeOpen = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function (method, url) {
    var args = Array.prototype.slice.call(arguments);
    args[1] = rewriteApiUrl(url);
    return nativeOpen.apply(this, args);
  };

  try {
    var storageKey = 'infinite-canvas:ai_config_store';
    var persisted = JSON.parse(window.localStorage.getItem(storageKey) || 'null');
    var config = persisted && persisted.state && persisted.state.config;
    if (!config) return;

    config.baseUrl = '/infinite-api';
    if (Array.isArray(config.channels)) {
      config.channels = config.channels.map(function (channel) {
        return Object.assign({}, channel, { baseUrl: '/infinite-api' });
      });
    }
    window.localStorage.setItem(storageKey, JSON.stringify(persisted));
  } catch (error) {
    // Ignore malformed or unavailable browser storage; the app can still load.
  }
})();
