const express = require('express');
const os = require('os');
const app = express();

app.use(express.json());

app.get('/', (req, res) => {
  res.send('Hello World!');
});

app.get('/env', (req, res) => {
  res.json(process.env);
});

app.get('/info', (req, res) => {
  const info = {
    hostname: os.hostname(),
    platform: os.platform(),
    arch: os.arch(),
    release: os.release(),
    uptime: os.uptime(),
    totalMemory: os.totalmem(),
    freeMemory: os.freemem(),
    cpus: os.cpus(),
    loadAverage: os.loadavg(),
    networkInterfaces: os.networkInterfaces(),
  };
  res.json(info);
});

app.listen(3000, () => {
  console.log('Server is running on port 3000');
});
