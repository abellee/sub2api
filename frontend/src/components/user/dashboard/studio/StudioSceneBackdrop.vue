<template>
  <div class="studio-scene-backdrop" aria-hidden="true">
    <i class="color-wash wash-blue"></i><i class="color-wash wash-orange"></i>
    <i class="tensor-tile tile-one"></i><i class="tensor-tile tile-two"></i><i class="tensor-tile tile-three"></i>
    <i class="spark spark-one"></i><i class="spark spark-two"></i><i class="data-dot dot-one"></i><i class="data-dot dot-two"></i>
  </div>
</template>
<style scoped>
.studio-scene-backdrop{position:absolute;inset:36px 0 24px;overflow:hidden;pointer-events:none;--wash-blue:#5f92fa29;--wash-orange:#ffa66522;--tile-ink:#798fe516}
:global(.dark .studio-scene-backdrop){--wash-blue:#5487f52c;--wash-orange:#ff985723;--tile-ink:#8b9be916}
.color-wash{position:absolute;width:75%;height:78%;border-radius:50%;filter:blur(20px)}.wash-blue{top:4%;left:0;background:radial-gradient(ellipse,var(--wash-blue),transparent 70%)}.wash-orange{right:-5%;bottom:0;background:radial-gradient(ellipse,var(--wash-orange),transparent 70%)}
.tensor-tile{position:absolute;border-radius:22%;background:linear-gradient(140deg,var(--tile-ink),transparent 85%);box-shadow:inset 1px 1px 0 #9baeee12;transform:rotate(-18deg) skew(8deg,5deg)}.tile-one{width:62px;height:62px;top:17%;left:10%}.tile-two{width:36px;height:36px;top:9%;right:17%;transform:rotate(20deg)}.tile-three{width:46px;height:46px;right:9%;bottom:21%;transform:rotate(-12deg)}
.spark{position:absolute;width:13px;height:13px;background:#99a9df55;clip-path:polygon(50% 0,62% 38%,100% 50%,62% 62%,50% 100%,38% 62%,0 50%,38% 38%)}.spark-one{top:37%;right:12%}.spark-two{top:13%;left:37%;width:8px;height:8px;background:#ffad7866}.data-dot{position:absolute;width:5px;height:5px;background:#9bb2df66;border-radius:50%}.dot-one{left:17%;bottom:29%}.dot-two{right:34%;top:22%;width:3px;height:3px}
</style>
