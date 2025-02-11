const express = require('express');
const os = require('os');
const fs = require('fs');
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
    osInfo: getOSInfo(),
    cgroupInfo: getCgroupsInfo(),
    namespacesInfo: getNamespacesInfo()
  };
  res.json(info);
});

function getOSInfo() {
  const osReleasePaths = ['/etc/os-release', '/usr/lib/os-release'];
  for (const path of osReleasePaths) {
    if (fs.existsSync(path)) {
      const data = fs.readFileSync(path, 'utf-8');
      const prettyName = findParameter(data, 'PRETTY_NAME');
      if (prettyName) {
        return prettyName.trim().replace(/"/g, '');
      }
    }
  }
  return 'UNKNOWN';
}

function findParameter(content, paramName) {
  const regex = new RegExp(`^${paramName}=(.*)$`, 'm');
  const match = content.match(regex);
  return match ? match[1] : null;
}

function getCgroupsInfo() {
  try {
    const cgroupSetup = fs.readFileSync('/proc/self/cgroup', 'utf-8');
    return cgroupSetup;
  } catch (err) {
    return 'Cgroups info unavailable';
  }
}

function getNamespacesInfo() {
  try {
    const namespacesConfig = fs.readFileSync('/proc/self/status', 'utf-8');
    const namespaces = namespacesConfig.split('\n').filter(line => line.startsWith('NSp')).map(line => line.trim());
    return namespaces;
  } catch (err) {
    return 'Namespaces info unavailable';
  }
}

app.listen(3000, () => {
  console.log('Server is running on port 3000');
});
