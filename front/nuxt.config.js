export default {
  mode: 'universal',
  server: {
    port: 3000,
    host: '0.0.0.0',
  },
  head: {
    title: process.env.npm_package_name || '',
    meta: [
      { charset: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      {
        hid: 'description',
        name: 'description',
        content: process.env.npm_package_description || '',
      },
    ],
  },
  loading: { color: '#fff' },
  css: [],
  plugins: [],
  buildModules: ['@nuxt/typescript-build'],
  modules: ['@nuxtjs/bulma', '@nuxtjs/axios', '@nuxtjs/pwa', '@nuxtjs/proxy'],
  axios: {},
  proxy: {
    '/api': {
      target: process.env.API_SERVER || 'http://server:8080',
      pathRewrite: {
        '^/api': '/',
      },
    },
  },
  build: {
    postcss: {
      preset: {
        features: {
          customProperties: false,
        },
      },
    },
    extend(_config, _ctx) {},
  },
}
