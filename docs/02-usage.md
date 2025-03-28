# Usage

The easiest way to use the `transceiver` service is by enabling *debug mode* in `roverctl-web`. This will automatically download, install and configure the `transceiver` service correctly.

Otherwise, the `transceiver` service can be configured through its *service.yaml* configuration. The name `transceiver` is given **special meaning** by `roverd` and any service named `transceiver` will always get all outputs from all other services injected in its [bootspec](https://ase.vu.nl/docs/framework/glossary/bootspec). You can find the conventions [here](https://ase.vu.nl/docs/framework/glossary/tuning).