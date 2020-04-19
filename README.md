# frodo <a href="https://pkg.go.dev/github.com/agnivade/frodo"> <img src="https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white"></a>

A quick POC to play with io_uring APIs using Go.

### Overview

Frodo deliberately keeps the API dead simple. Because he does not want the ring to fall into the wrong hands. It just exposes 2 very simple `ReadFile` and `WriteFile` functions which are akin to the ioutil family of functions. These calls just push an entry to the submission queue. To allow the user to control when to submit the queue, a `Poll` function is provided.

The `Poll` function will submit the queue and wait for all the entries to appear in the completion queue. Dive into the code to know more :)

For a more detailed background, please read: https://kernel.dk/io_uring.pdf.

### Pre-requisites

- Install liburing in your machine from latest master. (https://github.com/axboe/liburing/)
- You need to have a modern (>=5.3) Linux kernel. It may work on older kernels too. But I have not tested it.

### Getting started

See the [docs](https://pkg.go.dev/github.com/agnivade/frodo) page to get started.
