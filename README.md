## Go Bookshelf App on GAE

This is based on the sample that comes with the SDK documentation plus a few extra things:

+ Cache the landing page in Memcache
+ Pub/Sub based notifications when comments are added to a book
 
#### Dev/Deploy

```
$> make build
...
$> make deploy-worker
...
$> make deploy-app
...
```