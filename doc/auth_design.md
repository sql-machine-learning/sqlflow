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

<img src="figures/auth1.png">

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
    ClientEndpoint    string
    DBConnStr         string  // mysql://user:pass@127.0.0.1:3306
    Token             int64  // useful only in "side-car" design
}
```

Users can set auth information in SQLFlow extended SQL statement like:

```sql
SET CREDENTIAL username secretkey
```

**Note:** that SQLFlow depend on [SSO](https://en.wikipedia.org/wiki/Single_sign-on)
services and databases services and training cluster should be able to
use the same SSO service. We use SSO so that SQLFlow client can set only one
credential for all three services: the database engine, the training cluster and
SQLFlow server itself.

When a user logged in to the SSO service, the auth server
is responsible to fetch/generate database credentials and training cluster
credentials. These credentials are saved in the session object and can be recreated
when a user re-login.

Possible two implementations listed below can satisfy what SQLFlow needs:

### A Straight Forward Design

<img src="figures/auth2.png">

In this design, users send their requests to the "Auth Server" including
their auth information. The auth server then communicates with SSO service
to check if the credentials are legal. If so the auth server then send the
SQLFlow request to SQLFlow server, when the SQLFlow returns result, the auth
server pipe the result back to the client together with the session's token.

If one user is already logged in, then the client should have saved the token,
the auth server only checks if the token is legal.

In this design, the client here is an HTTP client rather than a gRPC client to
make this work. And the client must support save COOKIEs. For a quick implementation
we can use the solution [here](https://stackoverflow.com/questions/31554771/how-to-use-cookies-in-python-requests).


### "Side-Car" Design

<img src="figures/auth2.png">

While the straight forward way may introduce additional network communication,
the "side-car" method can avoid this but is more complicated.

In "side-car" design, a standalone auth server accepts client auth requests,
and broadcast the session information to SQLFlow server. The SQLFlow server will
store the session data for a short period of time. Then the auth server sends
token as a response to the client. Then the client can send requests directly
to SQLFlow gRPC server.

The limitation for this design is that, when adding new SQLFlow server instances
in the cluster, it may not know the current existing sessions. In order to make
SQFLow server stateless, we must store the sessions in another service like etcd.

## Conclusion

To make SQLFlow server production ready, supporting serve multiple clients on one
SQLFlow server instance is necessary, and in order to achieve this, we should
implement Authentication server and session management.

For simplicity, we only support services based on SSO, so that we won't need to
specify three types of credentials when users log in.
