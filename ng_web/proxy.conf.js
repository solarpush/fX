const target = process.env.API_PROXY_TARGET || 'http://localhost:8080';

module.exports = {
  '/api/v1/ws': {
    target,
    secure: false,
    changeOrigin: true,
    ws: true,
  },
  '/api/v1': {
    target,
    secure: false,
    changeOrigin: true,
    ws: true,
    timeout: 300000,
    proxyTimeout: 300000,
  },
  '/ws': {
    target,
    secure: false,
    changeOrigin: true,
    ws: true,
  },
  '/images': {
    target,
    secure: false,
    changeOrigin: true,
  },
};
