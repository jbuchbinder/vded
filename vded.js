// VDED - Vector Delta Engine Daemon
// https://github.com/jbuchbinder/vded
//
// vim: tabstop=4:softtabstop=4:shiftwidth=4:noexpandtab

var http = require('http');
var url  = require('url');
var fs   = require('fs');
var gm   = require('./gmetric');
var exec = require('child_process').exec;

// Default values, overriden by command arguments
var server_port = 48333;
var pidfile = "";
var statefile = "";
var ganglia_enabled = false;
var ganglia_host = null;
var ganglia_port = 8649;
var ganglia_spoof = null;
var max_entries = 0;
var vectors = {};
var switches = {};
var purgeInterval = 30 * 1000;
var flushInterval = 1 * 60 * 1000;

parseArgs(process.argv.splice(2));

// Write the PID to a file
writePid();

// Load data, if there is any, from state file
deserializeFromFile();

// Initialize gmetric sending, if enabled
var gmetric;
if (ganglia_enabled) {
	gmetric = new gm.gmetric( ganglia_host, ganglia_port, ganglia_spoof != null ? ganglia_spoof : null );
}

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
			vectors[key].latest_value = value;
			if (args['spoof'] != null) {
				vectors[key].spoof = args['spoof'];
			}
			if (args['units'] != null) {
				vectors[key].units = args['units'];
			}
			if (args['group'] != null) {
				vectors[key].group = args['group'];
			}
			if (submit_metric != null) {
				vectors[key].submit_metric = submit_metric;
			}
		} else {
			// Create new entry
			var v = {
				'host': hostname,
				'name': vector_name,
				'spoof': args['spoof'] ? args['spoof'] : true,
				'submit_metric': submit_metric == null ? true : submit_metric,
				'latest_value': value,
				'units': args['units'] ? args['units'] : 'count',
				'group': args['group'] ? args['group'] : 'vectors',
				'values': { }
			};
			v.values[ts] = value;
			vectors[key] = v;
		}

		var obj = buildVectorResponse(key);
		//console.log( "Current object value = " + JSON.stringify(obj) );
		//console.log( "obj.values.length = " + Object.keys(obj.values).length );
		if (Object.keys(obj.values).length > 1 && obj.submit_metric) {
			submitToGanglia( obj.host, obj.name, obj, obj['last_diff'] );
			if (obj['per_5min'] != null) {
				submitToGanglia( obj.host, obj.name + '_per_5min', obj, obj['per_5min'] );
			}
			if (obj['per_10min'] != null) {
				submitToGanglia( obj.host, obj.name + '_per_10min', obj, obj['per_10min'] );
			}
			if (obj['per_30min'] != null) {
				submitToGanglia( obj.host, obj.name + '_per_30min', obj, obj['per_30min'] );
			}
			if (obj['per_1hour'] != null) {
				submitToGanglia( obj.host, obj.name + '_per_hour', obj, obj['per_1hour'] );
			}
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

var flushProcess = setInterval(function () {
	// Save state to file.
	console.log("Serialize to file");
	var timer_begin = Date.now();
	serializeToFile( );
	var timer_end = Date.now();
	console.log("FLUSH|Completed in " + ( timer_end - timer_begin ) + " ms");
}, flushInterval);

// Start up purge process
var purgeProcess = setInterval(function () {
	console.log("PURGE|Start purge process");
	if (max_entries <= 0) {
		console.log("PURGE|No purge limit, exiting.");
		// Skip purging if we have no limit
		return;
	}

	var timer_begin = Date.now();

	for (var i in vectors) {
		var vectorvalues = Object.keys(vectors[i].values).length;
		if (vectorvalues <= max_entries) {
			// If we don't have enough entries, skip this vector
			continue;
		}

		var vKeys = Object.keys(vectors[i].values);
		vKeys.sort();

		// Slice off (NUM_ENTRIES - max_entries) - 1
		vKeys.slice(0, (vectorvalues - max_entries) - 1);
		var idx = 0;
		for (idx = 0; idx < vKeys.length; idx++) {
			var ts = vKeys[idx];
			console.log("PURGE|Vector " + vectors[i].host + "/" + vectors[i].name + " purging ts " + ts);
			delete vectors[i].values[ts];
		}
	}

	var timer_end = Date.now();
	console.log("PURGE|Completed in " + ( timer_end - timer_begin ) + " ms");
}, purgeInterval);

function onExit() {
	// Kill purge process if it's running
	if (purgeProcess != null) {
		console.log("Remove purge process");
		clearInterval( purgeProcess );
	}

	if (flushProcess != null) {
		console.log("Remove flush process");
		clearInterval( flushProcess );
	}

	if (ganglia_enabled) {
		try {
			gmetric.close();
		} catch (e) {
		}
	}

	// Serialize to file on shutdown
	console.log("Serialize to file");
	serializeToFile( );

	// Remove pid
	console.log("Remove PID");
	removePid();

	process.exit();
}

process.on('SIGINT', onExit);
process.on('SIGQUIT', onExit);

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
					console.log("Enable send to ganglia");
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
	var timer_begin = Date.now();
	var v = vectors[key];
	if (Object.keys(v.values).length == 1) {
		// Only one, return with no calculation
		v['last_diff'] = v.latest_value;
		v['per_minute'] = 0;
		v['per_hour'] = 0;
	} else {
		// Determine differences
		var keys = new Array();
		for (var i in v['values']) {
			keys.push(i);
		}
		keys.sort();
		var max1 = keys[keys.length - 1];
		var max2 = keys[keys.length - 2];
		var ts_diff = max1 - max2;
		if (ts_diff < 0) { ts_diff = -ts_diff; }
		if (v.values[max1] < v.values[max2]) {
			// Deal with vector value resets, not perfect, but good enough
			v['last_diff'] = v.values[max1];
		} else {
			// As normal, use actual difference
			v['last_diff'] = v.values[max1] - v.values[max2];
		}

		// These to track back in history to find the entries
		// which most closely match their timestamps, and use those
		// values to figure rate over time.
		var key5min  = getClosestAgeEntry(v, v['last_diff'],  300);
		if (key5min != null) {
			var diff = v['last_diff'] - key5min;
			v['per_5min'] = v.values[key5min] / ( diff / 300 );
		}
		var key10min = getClosestAgeEntry(v, v['last_diff'],  600);
		if (key5min != null) {
			var diff = v['last_diff'] - key10min;
			v['per_10min'] = v.values[key10min] / ( diff / 600 );
		}
		var key30min = getClosestAgeEntry(v, v['last_diff'], 1800);
		if (key30min != null) {
			var diff = v['last_diff'] - key10min;
			v['per_30min'] = v.values[key30min] / ( diff / 1800 );
		}
		var key1hour = getClosestAgeEntry(v, v['last_diff'], 3600);
		if (key1hour != null) {
			var diff = v['last_diff'] - key1hour;
			v['per_hour'] = v.values[key1hour] / ( diff / 3600 );
			v['per_1hour'] = v.values[key1hour] / ( diff / 3600 );
		}

		// Standard "per minute" calculation
		v['per_minute'] = (ts_diff < 30) ? 0 : v['last_diff'] / ( ts_diff / 60 );
	}
	var timer_end = Date.now();
	console.log("RESP|Completed in " + ( timer_end - timer_begin ) + " ms");
	return v;
}

function getClosestAgeEntry(vector, now, age) {
	var target = now - age;
	var found = null;
	var bestDiff = 9999999;

	// Grab list of keys (which double as timestamps)
	for (var j in vector.values) {
		if ( Math.abs( j - target ) < bestDiff ) {
			bestDiff = Math.abs( j - target );
			found = j;
		}
	}
	return found;
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

function serializeToFile( ) {
	if (statefile != '') {
		console.log("Serializing state to " + statefile);
		var obj = {};
		obj.vectors = vectors;
		obj.switches = switches;
		var out = JSON.stringify( obj );
		console.log( out );
		fs.writeFileSync( statefile, out, 'utf8' );
	}
}

function deserializeFromFile() {
	try {
		if (statefile != '' && fs.lstatSync(statefile)) {
			console.log("Retrieving state from " + statefile);
			var raw = fs.readFileSync( statefile, 'utf8' );
			console.log(raw);
			var obj = JSON.parse( raw );
			vectors = obj['vectors'] != null ? obj['vectors'] : { };
			switches = obj['switches'] != null ? obj['switches'] : { };
		}
	} catch (e) {
		console.log("State file does not exist, skipping deserialization.");
	}
}

function submitToGanglia( host, name, vector, value ) {
	console.log("submitToGanglia()");
	if (!ganglia_enabled) { return; }
	console.log("Send value " + value);

	// Hack to send using gmetric binary until node-gmetric works properly
	//var cmd = "/usr/bin/gmetric " + 
	//	" -g '" + ( vector.group != null ? vector.group : 'vectors' ) "' " +
	//	" -n '" + name + "' " +
	//	" -v '" + value + "' " +
	//	" -u '" + ( vector.units == null ? 'count' : vector.units ) + "' " +
	//	" -x 300 -t uint32 " +
	//	( ganglia_spoof != null ? " -S '" + ganglia_spoof + "'" : "" );
	//console.log("GMETRIC CMD: " + cmd);
	//exec( cmd, function (error, stdout, stderr) {
	//	if (error != null) {
	//		console.log("GMETRIC error  : " + error);
	//	}
	//	if (stdout != null || stderr != null) {
	//		console.log("GMETRIC stdout : " + stdout);
	//		console.log("GMETRIC stderr : " + stderr);
	//	}
	//});

	// Native gmetric support
	gmetric.sendMetric( host, name, value, vector.units == null ? 'count' : vector.units, gm.VALUE_INT, gm.SLOPE_BOTH, 300, 300, vector.group == null ? 'vectors' : vector.group );
}

console.log("VDED listening on port " + server_port);

