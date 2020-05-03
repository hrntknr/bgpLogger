<template>
  <div class="container">
    <div>
      <h1 class="top_title title is-1 has-text-centered">
        bgpLogger
      </h1>
      <form
        @submit.prevent="
          $router.push(`/search?ip=${encodeURIComponent(searchInput)}`)
        "
      >
        <div class="top_search field has-addons">
          <div class="top_search_box control">
            <input
              v-model="searchInput"
              class="input"
              type="text"
              placeholder="2001:200:0:1001::88"
            />
          </div>
          <div class="control">
            <button class="button is-info" type="submit">
              Search
            </button>
          </div>
        </div>
      </form>
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
          <tr v-for="(history, i) in all.history" :key="i">
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
  async asyncData({ $axios }) {
    const { data } = await $axios.get('/api/all')
    data.history.forEach((item: any) => {
      item.timestamp = dayjs(item['@timestamp'])
      item.diff = item.timestamp.fromNow()
    })
    return {
      all: data,
    }
  },
})
export default class Slug extends Vue {
  searchInput: string = ''
}
</script>

<style scoped>
.top_title {
  margin-top: 100px;
  margin-bottom: 100px !important;
}
.top_search {
  justify-content: center !important;
  margin-bottom: 100px !important;
}
.top_search_box {
  max-width: 600px;
  width: 100%;
}
</style>
