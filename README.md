# datayoinker

My attempt to replace dweet.io cause 5 dweets in the last 24 hours is not enough storage.

## Description

The service is designed to be easy to deploy and run.
For this reason, SQLite was picked as the database and the native Go driver for it is used instead of the GGo one.
This way we can keep the easy cross-compilation and static linking.
It's also available as a docker image, the only caveat is that I haven't fully tested the volume permissions.

## Usage

Since the main aim was to replace dweet.io, a similar HAPI-like API has been implemented.
In addition, a more standard REST-like API is currently being worked on.

### HAPI routes

```
/publish/yoink/for/{topic}
/get/latest/yoink/from/{topic}
/get/latest/{number}/yoinks/from/{topic}
/get/{number}/latest/yoinks/from/{topic}
/get/last/{number}/yoinks/from/{topic}
/get/{number}/last/yoinks/from/{topic}
/get/all/yoinks/from/{topic}
```

### REST routes

```
/yoink/{topic}
/yoink/{topic}
/yoinks/{topic}/{number}
/yoinks/{topic}
```
