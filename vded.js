// VDED - Vector Delta Engine Daemon
// https://github.com/jbuchbinder/vded
//
// vim: tabstop=4:softtabstop=4:shiftwidth=4:noexpandtab

var http = require('http');
var sys  = require('sys');
var url  = require('url');
var fs   = require('fs');
var gm   = require('gmetric');

// Default values, overriden by command arguments
var server_port = 48333;
var pidfile = "";
var statefile = "";
var ganglia_enabled = false;
var ganglia_host = null;
var ganglia_port = 8649;
var ganglia_spoof = null;
var max_entries = 0;
var vectors = [];
var switches = [];

parseArgs(process.argv.splice(2));

// Write the PID to a file
writePid();

// Load data, if there is any, from state file
deserializeFromFile();

// TODO: Thread to trim number of remembered entries for values to
// max_entries for switches and vectors

http.createServer(function(req,resp) {
	var uparse = url.parse(req.url, true);
	var path = uparse.pathname;
	var args = uparse.query;
	console.log("Request for " + path);
	if (path == '/test') {
		createResponse(resp, 200, ["This is a test",{'a':1,'b':2}]);
	} else if (path == '/favicon.ico') {
		createResponse(resp, 404, "Not found!");
	} else if (path == '/vector' || path == '/submit') {
		var hostname = args['host'] == null ? "localhost" : args['host'];
		var vector_name = args['vector'];
		if (vector_name == null) {
			createResponse(resp, 500, "Bad parameters (vector not given)");
			return;
		}

		var value = args['value'];
		if (value == null) {
			createResponse(resp, 500, "Bad parameters (value not given)");
			return;
		}

		var ts = args['ts'];
		if (ts == null) {
			createResponse(resp, 500, "Bad parameters (ts not given)");
			return;
		}

		var submit_metric = null;
		if (args['submit_metric'] != null) {
			var s = args['submit_metric'];
			if (s.toLowerCase() == 'true' || s == '1' || s.toLowerCase() == 'yes') {
				submit_metric = true;
			} else if (s.toLowerCase() == 'false' || s == '0' || s.toLowerCase() == 'no') {
				submit_metric = false;
			}
		}

		var key = getKeyName(hostname, vector_name);

		if (vectors[key] != null) {
			// Use old entry
			vectors[key].values[ts] = value;
			if (args['spoof'] != null) {
				vectors[key].spoof = args['spoof'];
			}
			if (submit_metric != null) {
				vectors[key].submit_metric = submit_metric;
			}
		} else {
			// Create new entry
			var v = {
				'host': hostname,
				'name': vector_name,
				'spoof': args['spoof'],
				'submit_metric': submit_metric == null ? true : submit_metric,
				'latest_value': value,
				'values': { }
			};
			v.values[ts] = value;
			vectors[key] = v;
		}

		var obj = buildVectorResponse(key);
		if (obj.values.length > 1 && vector.submit_metric) {
			submitToGanglia( vector.host, vector.name, vector, obj.latest_value );
		}
		createResponse(resp, 200, obj);
	} else if (path == '/switch') {
		var action = args['action'] == null ? "put" : args['action'];
		var hostname = args['host'] == null ? "localhost" : args['host'];
		var switch_name = args['switch'];
		if (switch_name == null) {
			createResponse(resp, 500, "Bad parameters (switch not given)");
			return;
		}
		if (action == 'put') {
			var value = args['value'];
			if (value == null) {
				createResponse(resp, 500, "Bad parameters (value not given)");
				return;
			}
			var actual_value = false;
			if (value.toLowerCase() == 'true' || value.toLowerCase() == 'on') {
				actual_value = true;
			} else if (value.toLowerCase() == 'false' || value.toLowerCase() == 'off') {
				actual_value = false;
			} else {
				createResponse(resp, 500, "Bad value for switch");
				return;
			}

			var ts = args['ts'];
			if (ts == null) {
				createResponse(resp, 500, "Bad parameters (ts not given)");
				return;
			}

			var submit_metric = true;
			if (args['submit_metric'] != null) {
				var s = args['submit_metric'];
				if (s.toLowerCase() == 'true' || s == '1' || s.toLowerCase() == 'yes') {
					submit_metric = true;
				} else if (s.toLowerCase() == 'false' || s == '0' || s.toLowerCase() == 'no') {
					submit_metric = false;
				}
			}

			var key = getKeyName(hostname, switch_name);

			if (switches[key] != null) {
				// Use old entry
				switches[key].values[ts] = actual_value;
				switches[key].latest_value = actual_value;
			} else {
				// Create new entry
				var s = {
					'host': hostname,
					'name': switch_name,
					'submit_metric': submit_metric,
					'latest_value': actual_value,
					'values': { }
				};
				s.values[ts] = actual_value;
				switches[key] = s;
			}
			createResponse(resp, 200, switches[key]);
		} else if (action == 'get') {
			var key = getKeyName(hostname, switch_name);
			if (switches[key] == null) {
				createResponse(resp, 401, "Switch does not exist");
				return;
			}
			createResponse(resp, 200, switches[key]);
			return;
		}
	} else {
		createResponse(resp, 404, "Not found!");
	}
}).listen(server_port);

process.on('exit', function() {
	// Save state to file.
	serializeToFile();

	// Remove pid
	removePid();
});

// Convenience functions

function parseArgs(argv) {
	var pos=0;
	var curarg='';
	for (pos=0; pos<argv.length; pos++) {
		//console.log(argv[pos]);
		if (curarg != '') {
			// Handle option for argument, if there is one
			switch (curarg) {
				case 'pid':
					pidfile = argv[pos];
					break;
				case 'port':
					server_port = argv[pos];
					break;
				case 'state':
					statefile = argv[pos];
					break;
				case 'ghost':
					ganglia_host = argv[pos];
					ganglia_enabled = true;
					break;
				case 'gport':
					ganglia_port = argv[pos];
					break;
				case 'gspoof':
					ganglia_spoof = argv[pos];
					break;
				case 'max':
					max_entries = argv[pos];
					break;
				default:
					showSyntax();
					break;
			}
			curarg = '';
		} else {
			switch (argv[pos]) {
				case '-P':
				case '--pid':
					curarg = 'pid';
					break;
				case '-p':
				case '--port':
					curarg = 'port';
					break;
				case '-s':
				case '--state':
					curarg = 'state';
					break;
				case '-G':
				case '--ghost':
					curarg = 'ghost';
					break;
				case '-g':
				case '--gport':
					curarg = 'gport';
					break;
				case '-S':
				case '--gspoof':
					curarg = 'gspoof';
					break;
				case '-m':
				case '--max':
					curarg = 'max';
					break;
				case '-h':
				case '--help':
				default:
					showSyntax();
					break;
			}
		}
	}
	if (curarg != '') {
		showSyntax();
	}
}

function showSyntax() {
	console.log("VDED - Vector Delta Engine Daemon");
	console.log("  -h|--help              Syntax help");
	console.log("  -P|--pid PIDFILE       Path for pid file");
	console.log("  -p|--port PORT         Listening port (default is 48333)");
	console.log("  -s|--state STATEFILE   Path for save state file");
	console.log("  -G|--ghost HOST        Ganglia hostname");
	console.log("  -g|--gport PORT        Ganglia port");
	console.log("  -S|--gspoof IP:HOST    Ganglia default spoof");
	console.log("  -m|--max INT           Maximum number of entries to retain");
	process.exit();
}

function createResponse(resp, status, obj) {
	resp.writeHead(status, {'Content-Type': 'text/plain'});
	resp.write(JSON.stringify(obj));
	resp.end();
}

function buildVectorResponse(key) {
	var v = vectors[key];
	if (v.values.length == 1) {
		// Only one, return with no calculation
		v['last_diff'] = v.latest_value;
		v['per_minute'] = 0;
		v['per_hour'] = 0;
	} else {
		// Determine differences
		var keys = new Array();
		for (var i in v.values) {
			keys.push(i);
		}
		keys.sort();
		var max1 = keys[keys.length - 1];
		var max2 = keys[keys.length - 2];
		var ts_diff = max1 - max2;
		if (ts_diff < 0) { ts_diff = -ts_diff; }
		v['last_diff'] = v.values[max1] - v.values[max2];
		v['per_minute'] = (ts_diff < 30) ? 0 : v['last_diff'] / ( ts_diff / 60 );
		v['per_hour'] = (ts_diff < 1800) ? 0 : v['last_diff'] / ( ts_diff / 3600 );
	}
	return v;
}

function getKeyName(host, value) {
	return host + '/' + value;
}

function writePid() {
	if (pidfile != '') {
		fs.writeFile( pidfile, process.pid );
	}
}

function removePid() {
	if (pidfile != '') {
		fs.unlinkSync( pidfile );
	}
}

function serializeToFile() {
	if (statefile != '') {
		console.log("Serializing state to " + statefile);
		var obj = {
			vectors: vectors,
			switches: switches
		};
		fs.writeFile( statefile, JSON.stringify(obj) );
	}
}

function deserializeFromFile() {
	if (statefile != '' && fs.lstat(statefile).isFile()) {
		console.log("Retrieving state from " + statefile);
		var obj = JSON.parse( fs.readFileSync( statefile ) );
		vectors = obj['vectors'];
		switches = obj['switches'];
	}
}

function submitToGanglia( host, name, vector, value ) {
	if (!ganglia_enabled) { return; }
	var g = new gm.gmetric( ganglia_host, ganglia_port, vector.spoof != null ? vector.spoof : ganglia_spoof );
	g.sendMetric( host, name, value, "count", gm.VALUE_INT, gm.SLOPE_UNSPECIFIED, 300, 300 );
}

console.log("VDED listening on port " + server_port);

