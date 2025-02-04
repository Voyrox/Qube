const express = require('express');
const app = express();

app.use(express.json());

app.get('/', (request, response) => {
  response.send('Hello World!');
});

app.post('/demo-object', (request, response) => {

  const body = request.body;

  response.json(body);
});

app.get('/demo-object/:id', (request, response) => {
  
  const params = request.params;

  return response.json(params);
});

app.get('/demo-object', (request, response) => {
  
  const params = request.query;

  return response.json(params);
});

app.listen(3000, () => {
  console.log('Server is running on port 3000');
});
