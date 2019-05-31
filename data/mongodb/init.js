db.createUser({
  user: "docker",
  pwd: "v3ry53cur3p455w0rd",
  roles: [
    { role: "dbAdmin", db: "tanglemonitor" },
    { role: "readWrite", db: "tanglemonitor" }
  ]
});
