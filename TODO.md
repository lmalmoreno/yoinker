# TODOs
Things left to do until I consider the project at least mostly done.

## Tests
Add tests for the functions/handlers that don't have them.
The untested ones are unexported, denoted by starting with a lower-case letter.

## Extra functionality
Apart from the base functionality, there are things that can be better.

### Set Content-Type HTTP header
The `Content-Type` header isn't currently being set as it should.

### JSON in request body
Having only query parameters is kinda lame, if data from the body could also be accounted for, that'd be neat.

### Registered topic
Add "registered" topic and make normal topics be deleted on a schedule.
Access to publish on registered topic should be able to happen using JWT or pre-configured password (user's choice).

### Scheduled data deletion
This will probably require a change to the schema.
Topics and yoinks associated with them should probably be moved to different tables.
Since sqlite doesn't include a way to run scheduled stuff, this will be a design challenge.

### Chunked HTTP response
To offer real-time data streaming support look into chunked http responses.

### Normal API
HAPI is *okay* but I'd like a normal REST-like API too
