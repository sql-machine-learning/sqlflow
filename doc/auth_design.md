# Design: SQLFlow Authentication and Authorization

## Terminology and Background

For a quick explanation, the word "Authentication" means to identify the
user recognized by the system is the actual real-world user. And "Authorization"
means to grant privileges to the user to access some part of the system
functionalities.

SQLFlow works as a "bridge" between databases and
Deep Learning/Machine Learning frameworks. In order to execute a job,
SQLFlow need both permissions to access databases and submit jobs to
systems to run distributed training jobs, like submitting jobs to Kubernetes
to run a distributed tensorflow job.

In production environments, one SQLFlow server is designed to accept many clients'
connections and job submissions. In this case, we must securely store a mapping
from the user's ID to the user's credentials for accessing both the database and the
training cluster. Then server-side "session" should also be considered during
implementing authentication.

For authorization, it will be definitely reasonable to directly proxy requests to
databases and training clusters, the request will be denied if the current user
have no access to the requested service.

## Design

An authentication server (for short, will use "auth server" instead) will be introduced
to achieve extensible authentication configurations. We use a
[Django](https://www.djangoproject.com/) web server so that the authentication methods
can extend to:

- Database authentication
- LDAP
- User-defined authentication methods

### Session

A server-side "session" is needed to store credentials for each client to access
the database and submitting jobs. The session can be defined as:

```go
type Session struct {
    Token          int64  // useful only in "side-car" design
    ClientEndpoint string // ip:port from the client
    DBConnStr      string // mysql://127.0.0.1:3306
    AK             string // access key
    SK             string // secret key
}
```

The token will act as the unique id of the session. The session object
should be expired within some time and deleted on the server memory.

We want to make sure that SQLFlow servers are stateless so that we can
deploy it on any cluster that does auto fail-over and auto-scaling. In
that case, we store session data into a reliable storage service like
[etcd](https://github.com/etcd-io/etcd). 

Possible two implementations listed below can satisfy what SQLFlow needs:

### Authentication of SQLFlow Server

**Note:** that SQLFlow should be dealing with three kinds of services:

- SQLFlow service itself
- Database service that stores the training data
- A training cluster that runs the SQLFlow training job, e.g. Kubernetes

SQLFlow should depend on the [SSO](https://en.wikipedia.org/wiki/Single_sign-on)
service. Databases and training clusters also need to check
if the user is valid and check if the user has granted proper permissions,
but these services may have different credentials other than the SSO service.
So there **must** be an "Auth Server" to fetch/create the user's AK/SK (access key/secret key)
which will be used by databases or Kubernetes.

For one case that we use MySQL as the database engine, the fetched AK/SK should
be the MySQL's user and password. When running on the cloud environment, AK/SK
should be the real user's keys.

<img src="figures/sqlflow_auth.png">

Users can use SQLFlow server with a simple jupyter notebook for simple deployment,
for production deployments, users can take advantage of the cloud web IDE. The web
IDE will redirect a user to the SSO service if the user is not logged in.

Once the user is logged in, SSO service will return the "token" represents the user's
identity. Then the web IDE will call the "Auth Service" to get AK/SK for the database and
training cluster. After that, the web IDE will call SQLFlow RPC service to create
a new session, and the SQLFlow server will verify that all tokens, AK/SK are valid, then
the session will be stored.

If one user is already logged in, then the web IDE should have saved the token,
then SQLFlow server can get the session to run jobs if the session not expired.

After all that, SQLFlow server works as usual except generated training jobs can
get all the credentials used for accessing databases or training clusters.


## Conclusion

To make SQLFlow server production ready, supporting serve multiple clients on one
SQLFlow server instance is necessary, Authentication and session management should
be implemented.

For production use, other services like web IDE, SSO, and Auth server are also needed
to protect user's data and computing quotas.
