# Overview

The `transceiver` service is a special service that allows for tuning and debugging of all services in a pipeline. You can read about the process of tuning and debugging [here](https://ase.vu.nl/docs/framework/glossary/tuning).

## Debugging

The `transceiver` captures [service output messages](https://github.com/VU-ASE/rovercom/blob/main/definitions/outputs/wrapper.proto) and encapsulates them in [debug output messages](https://github.com/VU-ASE/rovercom/blob/main/definitions/debug/debug.proto) before sending them off to the `roverctl` webRTC proxy. 

## Tuning

The `transceiver` receives [tuning state messages](https://github.com/VU-ASE/rovercom/blob/main/definitions/outputs/wrapper.proto) from the `roverctl` webRTC proxy and outputs them on its **tuning** write stream for all services to read. Services can then use the tuning states to update their internal state (as done by the [*roverlib*](https://ase.vu.nl/docs/framework/glossary/roverlib)).