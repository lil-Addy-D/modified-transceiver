<h1 align="center"><code>transceiver</code> service</h1>
<div align="center">
  <a href="https://github.com/VU-ASE/transceiver/releases/latest">Latest release</a>
  <span>&nbsp;&nbsp;•&nbsp;&nbsp;</span>
  <a href="https://ase.vu.nl/docs/category/transceiver">Documentation</a>
  <br />
</div>
<br/>

**The `transceiver` service listens to the outputs of all other services in a pipeline and forwards this data to the `roverctl` webRTC proxy for debugging purposes. Conversely, it listens to the `roverctl` webRTC proxy for [tuning state messages](https://github.com/VU-ASE/rovercom/blob/main/definitions/tuning/tuning.proto) that it forwards to all services in the pipeline to enable [tuning and debugging](/docs/framework/glossary/tuning).**
