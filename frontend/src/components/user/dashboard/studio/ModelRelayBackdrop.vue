<template>
  <svg class="relay-backdrop" viewBox="0 0 850 460" fill="none" aria-hidden="true" preserveAspectRatio="xMidYMid slice">
    <g class="relay-connections">
      <path d="M35 126H91Q112 126 125 146L184 223Q197 240 221 240H301M65 360H123Q144 360 159 339L207 272Q218 254 241 254H301M114 54V95Q114 111 132 111H235Q257 111 257 135V187Q257 207 280 207H323" />
      <path d="M360 219C439 219 443 70 524 70H715M360 239C443 239 457 159 531 159H795M360 258C447 258 460 329 531 329H763M360 278C424 278 449 408 526 408H689" />
      <path d="M524 70L557 159L531 329L526 408M615 70L650 159L640 329L613 408M715 70L737 159L763 329L689 408" stroke-dasharray="2 7" />
      <path d="M132 111L184 223L123 360M235 111L221 240L207 272M524 70L650 159L531 329L689 408M715 70L557 159L763 329L613 408" class="neural-links" />
    </g>
    <g class="relay-nodes"><circle v-for="point in nodes" :key="point.join('-')" :cx="point[0]" :cy="point[1]" r="3.5" /></g>
    <g class="relay-chip"><rect x="302" y="201" width="58" height="91" rx="15" /><path d="M317 235L330 222L344 235V257L330 270L317 257Z" /><path d="M323 243H337M330 236V250" /></g>
    <g class="relay-packets"><circle cx="465" cy="114" r="4" /><circle cx="456" cy="189" r="3" /><circle cx="454" cy="301" r="4" /><circle cx="171" cy="312" r="3" /></g>
  </svg>
</template>

<script setup lang="ts">
const nodes = [[35,126],[65,360],[114,54],[132,111],[184,223],[123,360],[235,111],[221,240],[207,272],[524,70],[557,159],[531,329],[526,408],[615,70],[650,159],[640,329],[613,408],[715,70],[737,159],[763,329],[689,408],[795,159]]
</script>

<style scoped>
.relay-backdrop{position:absolute;inset:0;width:100%;height:100%;pointer-events:none;color:var(--studio-grid);opacity:.55;mask-image:linear-gradient(90deg,#000 0%,#000 70%,transparent 100%)}.relay-connections{stroke:currentColor;stroke-width:.85}.neural-links{opacity:.35}.relay-nodes{fill:var(--tf-console-surface);stroke:currentColor;stroke-width:1}.relay-chip{stroke:currentColor;stroke-width:1;fill:var(--tf-console-surface)}.relay-chip path{fill:none;stroke:var(--tf-console-active-ink);opacity:.45}.relay-packets{fill:var(--tf-console-active-ink);opacity:.35}
</style>
