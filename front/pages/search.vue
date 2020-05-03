<template>
  <div class="container">
    <div>
      <h2 class="search_title title is-3 has-text-centered">
        Route history of {{ $route.query.ip }}
      </h2>
      <table class="table is-fullwidth">
        <thead>
          <tr>
            <th>Type</th>
            <th>timestamp</th>
            <th>Prefix</th>
            <th>AS_PATH</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(history, i) in history.history" :key="i">
            <td>
              {{ history.type }}
            </td>
            <td>{{ history.timestamp }} ({{ history.diff }})</td>
            <td>
              {{ history.prefix }}
            </td>
            <td>
              {{ history.as_path }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script lang="ts">
import { Vue, Component } from 'nuxt-property-decorator'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
dayjs.extend(relativeTime)

@Component({
  async asyncData({ $axios, route }) {
    if (typeof route.query.ip !== 'string') {
      return
    }
    const { data } = await $axios.get(
      `/api/searchByIP?ip=${encodeURIComponent(route.query.ip)}`,
    )
    data.history.forEach((item: any) => {
      item.timestamp = dayjs(item['@timestamp'])
      item.diff = item.timestamp.fromNow()
    })
    return {
      history: data,
    }
  },
})
export default class Slug extends Vue {
  searchInput: string = ''
}
</script>

<style scoped>
.search_title {
  margin-top: 20px;
  margin-bottom: 20px;
}
</style>
