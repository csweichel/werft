const shell = require('shelljs');
const fs = require('fs');

const context = JSON.parse(fs.readFileSync('context.json'));
console.log(context);