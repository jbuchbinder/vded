#!/usr/bin/node
// VDED - Vector Delta Engine Daemon
// https://github.com/jbuchbinder/vded
//
// File format converter from node.js based output to Go based output
//
// vim: tabstop=4:softtabstop=4:shiftwidth=4:noexpandtab

var fs   = require('fs');

var statefile = "state.json.orig";
var outfile   = "state.json";

console.log("Retrieving state from " + statefile);
var raw = fs.readFileSync( statefile, 'utf8' );
//console.log( raw );
var obj = JSON.parse( raw );

var vkeys = Object.keys(obj.vectors);
vkeys.sort();

var totalProcessedVectors = 0;
var totalProcessedVectorValues = 0;

for (var idx = 0; idx < vkeys.length; idx++) {
  var vk = vkeys[idx];
  console.log("Processing " + vk);
  obj.vectors[vk].last_diff = parseInt(obj.vectors[vk].last_diff);
  obj.vectors[vk].latest_value = parseInt(obj.vectors[vk].latest_value);
  var c = 0; 
  var keys = Object.keys(obj.vectors[vk].values);
  keys.sort();
  for (var kidx = 0; kidx < keys.length; kidx++) {
    c++;
    var k = keys[kidx];
    obj.vectors[vk].values[k] = parseInt(obj.vectors[vk].values[k]);
  }
  console.log(" --> Processed " + c + " values");
  totalProcessedVectorValues += c;
  totalProcessedVectors++;
}

console.log("Total processed vectors: " + totalProcessedVectors);
console.log("Total processed values : " + totalProcessedVectorValues);

console.log("Serializing state to " + outfile);
var out = JSON.stringify( obj );
fs.writeFileSync( outfile, out, 'utf8' );

