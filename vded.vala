/**
 * VDED
 * https://github.com/jbuchbinder/vded
 *
 * vim: tabstop=4:softtabstop=4:shiftwidth=4:expandtab
 */

using GLib;
using Json;
using Posix;
using Soup;

class Vded {

    public class Switch : GLib.Object {
        public string host { get; set construct; }
        public string name { get; set construct; }
        public Gee.HashMap<long, bool> values { get; set construct; }
        public bool latest_value { get; set construct; }

        public void init_values() {
            values = new Gee.HashMap<long, bool>();
        } // end init_values

        public static Switch from_json(Json.Object o) {
            Switch v = new Vded.Switch();
            v.init_values();
            v.host = o.get_string_member("host");
            v.name = o.get_string_member("name");
            v.latest_value = o.get_boolean_member("latest_value");
            var values = o.get_object_member("values");
            foreach ( string k in values.get_members() ) {
                v.values.set(long.parse(k), values.get_boolean_member(k));
            }
            return v;
        } // end from_json

        public Json.Object to_json() {
            var o = new Json.Object();
            o.set_string_member("host", host);
            o.set_string_member("name", name);

            var a = new Json.Object();
            foreach (long k in values.keys) {
                a.set_boolean_member(k.to_string(), values[k]);
            }
            o.set_object_member("values", a);

            o.set_boolean_member("latest_value", latest_value);

            return o;
        } // end to_json

        public string to_string() {
            return "Vded.Switch[host=" + ( host != null ? host : "null" ) +
                ",name=" + ( name != null ? name : "null" ) +
                ",values=" + ( values != null ? "values" : "null" ) + "]";
        } // end to_string

        public void add_value(long ts, bool v) {
            latest_value = v;
            if (values == null) { init_values(); }
            values.set(ts, v);
        } // end add_value

    } // end class Switch

    public class Vector : GLib.Object {
        public string host { get; set construct; }
        public string name { get; set construct; }
        public Gee.HashMap<long, string> values { get; set construct; }
        public uint64 latest_value { get; set construct; }

        public void init_values() {
            values = new Gee.HashMap<long, string>();
        } // end init_values

        public static Vector from_json(Json.Object o) {
            Vector v = new Vded.Vector();
            v.init_values();
            v.host = o.get_string_member("host");
            v.name = o.get_string_member("name");
            v.latest_value = uint64.parse(o.get_string_member("latest_value"));
            var values = o.get_object_member("values");
            foreach ( string k in values.get_members() ) {
                v.values.set(long.parse(k), values.get_string_member(k));
            }
            return v;
        } // end from_json

        public Json.Object to_json() {
            var o = new Json.Object();
            o.set_string_member("host", host);
            o.set_string_member("name", name);

            var a = new Json.Object();
            foreach (long k in values.keys) {
                a.set_string_member(k.to_string(), values[k]);
            }
            o.set_object_member("values", a);

            o.set_string_member("latest_value", latest_value.to_string());

            return o;
        } // end to_json

        public string to_string() {
            return "Vded.Vector[host=" + ( host != null ? host : "null" ) +
                ",name=" + ( name != null ? name : "null" ) +
                ",values=" + ( values != null ? "values" : "null" ) + "]";
        } // end to_string

        public void add_value(long ts, string v) {
            uint64 realval = uint64.parse(v);
            latest_value = realval;
            if (values == null) { init_values(); }
            values.set(ts, v);
        } // end add_value

    } // end class Vector

    // Global variables
    protected Gee.HashMap<string, Vded.Switch> switches;
    protected Gee.HashMap<string, Vded.Vector> vectors;
    protected Soup.Server rest_server;

    protected bool daemonize = false;
    protected static bool debug = false;
    public static string lock_file;
    public static string state_file;

    protected bool ganglia_enabled = false;
    protected string ganglia_host;
    protected int ganglia_port;
    protected string? ganglia_spoof;

    public static void main (string[] args) {
        // Initialize syslog
        Posix.openlog( "vded", LOG_CONS | LOG_PID, LOG_LOCAL0 ); 

        Vded c = new Vded();
        c.init(args);
    } // end main

    Vded() {
        switches = new Gee.HashMap<string, Vded.Switch>();
        vectors = new Gee.HashMap<string, Vded.Vector>();
    }

    ~Vded() {
        // Handle gracefully shutting down the REST server.
        if (rest_server != null) {
            rest_server.quit();
        }
        if (state_file != null) {
            serialize();
        }
    } // end destructor

    public string get_key_name(string host, string key_name) {
        return host + "/" + key_name;
    } // end keyname

    public void syntax() {
        print("VDED ( https://github.com/jbuchbinder/vded )\n" +
            "\n" +
            "Flags:\n" +
            "\t-d             daemonize\n" +
            "\t-v             verbose\n" +
            "\t-l FILE        specify lockfile\n" +
            "\t-s FILE        specify state file\n" +
            "\t-G HOST        specify ganglia host (enables ganglia export)\n" +
            "\t-g HOST        specify ganglia port\n" +
            "\t-S SPOOF       specify ganglia spoof (IP:hostname)\n" +
            "\n");
        syslog(LOG_ERR, "Syntax error encountered parsing command line arguments.");
        exit(1);
    }

    public void init (string[] args) {
        // Parse args
        for (int i=1; i<args.length; i++) {
            if (args[i] == "-d") {
                daemonize = true;
            } else if (args[i] == "-v") {
                debug = true;
            } else if (args[i] == "-G") {
                if (args.length >= i+1) {
                    ganglia_host = args[++i];
                    ganglia_enabled = true;
                } else {
                    syntax();
                }
            } else if (args[i] == "-g") {
                if (args.length >= i+1) {
                    ganglia_port = int.parse(args[++i]);
                } else {
                    syntax();
                }
            } else if (args[i] == "-S") {
                if (args.length >= i+1) {
                    ganglia_spoof = args[++i];
                } else {
                    syntax();
                }
            } else if (args[i] == "-l") {
                if (args.length >= i+1) {
                    lock_file = args[++i];
                } else {
                    syntax();
                }
            } else if (args[i] == "-s") {
                if (args.length >= i+1) {
                    state_file = args[++i];
                    syslog(LOG_DEBUG, "State file = " + state_file + "\n");
                } else {
                    syntax();
                }
            } else {
                // Handle unknown arguments
                syntax();
            }
        }

        // If we specified a state file and it exists, load
        if (state_file != null && FileUtils.test(state_file, GLib.FileTest.EXISTS) == true) {
            deserialize();
        }

        if (daemonize) {
            // FIXME: reenable when this works. :(
            //daemonize_server();
        }

        // Initialize REST server thread to deal with inbound
        // requests from other parts of the system.
        init_rest_server();

        GLib.MainLoop loop = new GLib.MainLoop();
        loop.run();
    }

    public void rest_callback (Soup.Server server, Soup.Message msg,
            string path, GLib.HashTable<string,string>? query,
            Soup.ClientContext client) {
        if (path == "/") {
            // Root path, show some informational page or send 404
            if (debug) print("root path requested!\n");
        } else if (path == "/favicon.ico") {
            // TODO: serve up a friendly favicon?
        } else if (path == "/switch") {
            //print("switch requested!\n");
            if (query == null) {
                //print("rest error?\n");
                rest_error(msg, 1, "Bad parameters");
            }
            string action = (query.get("action") == null) ? "put" : query.get("action");

            string hostname = (query.get("host") == null) ? "localhost" : query.get("host");
            string switch_name = query.get("switch");
            if (switch_name == null) {
                rest_error(msg, 2, "Bad parameters (switch not given)");
                return;
            }

            if (action == "put") {
                string value = query.get("value");
                if (value == null) {
                    rest_error(msg, 2, "Bad parameters (value not given)");
                    return;
                }
                bool actual_value = false;
                if (value == "true" || value == "TRUE" || value == "on" || value == "ON") {
                    actual_value = true;
                } else if (value == "false" || value == "FALSE" || value == "off" || value == "OFF") {
                    actual_value = false;
                } else {
                    rest_error(msg, 2, "Bad value for switch");
                    return;
                }

                string ts = query.get("ts");
                if (ts == null) {
                    rest_error(msg, 2, "Bad parameters (ts not given)");
                    return;
                }

                // Look up key
                string key = get_key_name(hostname, switch_name);
                if (debug) print("key = %s\n", key);

                Vded.Switch v = null;
                bool create_new = false;
                if (!switches.has_key(key)) {
                    if (debug) print("Need to create new key %s\n", key);
                    create_new = true;
                    v = new Vded.Switch();
                    v.init_values();
                    v.host = hostname;
                    v.name = switch_name;
                } else {
                    if (debug) print("Found switch " + key + ", retrieving\n");
                    v = switches.get(key);
                }

                // Add value to list of switches
                if (debug) print("Add value\n");
                v.add_value(long.parse(ts), actual_value);

                if (create_new) {
                    if (debug) print("Storing new key\n");
                    switches.set(key, v);
                }

            } else if (action == "get") {
                // Look up key
                string key = get_key_name(hostname, switch_name);
                if (debug) print("key = %s\n", key);

                if (!switches.has_key(key)) {
                    rest_error(msg, 2, "Switch does not exist");
                    return;
                }
                Vded.Switch v = switches.get(key);

                if (debug) print("Build return values\n");
                build_switch_return_values(msg, v);
            }
        } else if (path == "/submit" || path == "/vector") {
            //print("submit requested!\n");
            if (query == null) {
                //print("rest error?\n");
                rest_error(msg, 1, "Bad parameters");
            }

            string hostname = (query.get("host") == null) ? "localhost" : query.get("host");
            string vector_name = query.get("vector");
            if (vector_name == null) {
                rest_error(msg, 2, "Bad parameters (vector not given)");
                return;
            }
            string value = query.get("value");
            if (value == null) {
                rest_error(msg, 2, "Bad parameters (value not given)");
                return;
            }
            string ts = query.get("ts");
            if (ts == null) {
                rest_error(msg, 2, "Bad parameters (ts not given)");
                return;
            }

            // DEBUG
            //foreach (string k in query.get_keys()) {
            //    print("k = %s, v = %s\n", k, query.get(k));
            //}

            // Look up key
            string key = get_key_name(hostname, vector_name);
            if (debug) print("key = %s\n", key);

            Vded.Vector v = null;
            bool create_new = false;
            if (!vectors.has_key(key)) {
                if (debug) print("Need to create new key %s\n", key);
                create_new = true;
                v = new Vded.Vector();
                v.init_values();
                v.host = hostname;
                v.name = vector_name;
            } else {
                if (debug) print("Found vector " + key + ", retrieving\n");
                v = vectors.get(key);
            }

            // Add value to list of vectors
            if (debug) print("Add value\n");
            v.add_value(long.parse(ts), value);

            if (create_new) {
                if (debug) print("Storing new key\n");
                vectors.set(key, v);
            }

            if (debug) print("Build return values\n");
            build_vector_return_values(msg, v);

        } else {
            if (debug) print("path = %s\n", path);
            msg.set_status(404);
            msg.set_response("text/plain", Soup.MemoryUse.COPY, "Path not found.".data);
        }
    } // end rest_callback

    public async void submit_to_ganglia (string host, string name, string value) {
        if (!ganglia_enabled)
            return;

        Gmetric gm = new Gmetric.new_metric( ganglia_host, ganglia_port, ganglia_spoof );
        gm.send_metric( host, name, value, "count", Gmetric.ValueType.VALUE_INT, Gmetric.Slope.UNSPECIFIED, 300, 300);
    } // end submit_to_ganglia

    public void build_switch_return_values (Soup.Message msg, Vded.Switch s) {
        if (s.values.size == 0) {
            rest_error(msg, 3, s.to_string() + " has no values");
            return;
        }

        var gen = new Json.Generator();
        var root = new Json.Node( NodeType.OBJECT );
        var object = new Json.Object();
        root.set_object(object);
        gen.set_root(root);

        object.set_boolean_member("last_value", s.latest_value);

        size_t length;
        string response = gen.to_data(out length);
        msg.set_status(200);
        msg.set_response("application/json", Soup.MemoryUse.COPY, response.data);
    } // end build_vector_return_values

    public void build_vector_return_values (Soup.Message msg, Vded.Vector vector) {
        if (vector.values.size == 0) {
            rest_error(msg, 3, vector.to_string() + " has no values");
            return;
        }

        uint64 last_diff = 0;
        long ts_diff = 0;
        uint64 per_minute = 0;
        uint64 per_hour = 0;

        // TODO : sort set, etc
        if (vector.values.size == 1) {
            // Only one, return just that, no calculation
            last_diff = vector.latest_value;
        } else {
            // Need to determine differences
            long[] keys = vector.values.keys.to_array();
            long max1 = keys[keys.length - 1];
            long max2 = keys[keys.length - 2];

            ts_diff = max1 - max2;
            if (ts_diff < 0) { ts_diff = -ts_diff; }

            if (debug) print("last_diff\n");
            if (debug) print(vector.values.get(max1) + " - " + vector.values.get(max2) + "\n");
            last_diff = uint64.parse(vector.values.get(max1)) - uint64.parse(vector.values.get(max2));
            if (debug) print("per min\n");
            per_minute = (ts_diff == 0 || ts_diff < 30) ? 0 : last_diff / ( ts_diff / 60 );
            if (debug) print("per hour\n");
            per_hour = (ts_diff == 0 || ts_diff < 1800) ? 0 : last_diff / ( ts_diff / 3600 );
        }

        var gen = new Json.Generator();
        var root = new Json.Node( NodeType.OBJECT );
        var object = new Json.Object();
        root.set_object(object);
        gen.set_root(root);

        object.set_string_member("last_diff", last_diff.to_string());
        object.set_string_member("per_minute", per_minute.to_string());
        object.set_string_member("per_hour", per_hour.to_string());

        // Push values to ganglia if enabled
        submit_to_ganglia(vector.host, vector.name, last_diff.to_string());

        size_t length;
        string response = gen.to_data(out length);
        msg.set_status(200);
        msg.set_response("application/json", Soup.MemoryUse.COPY, response.data);
    } // end build_vector_return_values

    public void rest_error (Soup.Message msg, int errno, string errmsg) {
        string response = "{\"response_type\": \"ERROR\", \"errno\": %d, \"message\": \"%s\"}".printf(errno, errmsg);
        msg.set_status(500);
        msg.set_response("application/json", Soup.MemoryUse.COPY, response.data);
    } // end rest_error

    public void init_rest_server() {
        rest_server = new Soup.Server(Soup.SERVER_PORT, 48333);
        //rest_server.port = 48333;
        rest_server.add_handler("/", rest_callback);
        rest_server.run_async();
    } // end init_rest_server

    public static void signal_handler ( int signum ) {
        syslog(LOG_ERR, signum.to_string() + " caught");
        if (lock_file != null) {
            Posix.unlink(lock_file);
        }
        exit(1);
    } // end signal_handler

    public void deserialize() {
        var parser = new Json.Parser ();
        try {
            string file_contents;
            FileUtils.get_contents(state_file, out file_contents);
            parser.load_from_data(file_contents, -1);
            var root_object = parser.get_root().get_object();
            var vectors_object = root_object.get_object_member("vectors");
            foreach (string vector in vectors_object.get_members()) {
                vectors.set(vector, Vector.from_json(vectors_object.get_object_member(vector)));
            }
        } catch (GLib.FileError e) {
            syslog(LOG_ERR, e.message);
        } catch (GLib.Error e) {
            syslog(LOG_ERR, e.message);
        }
    } // end deserialize

    public void serialize() {
        var gen = new Json.Generator();
        var root = new Json.Node( NodeType.OBJECT );
        var object = new Json.Object();
        root.set_object(object);
        gen.set_root(root);

        var vectors = new Json.Object();

        foreach (string k in this.vectors.keys) {
            // Add back to list of vectors
            vectors.set_object_member(k, this.vectors[k].to_json());
        }

        object.set_object_member("vectors", vectors);

        size_t length;
        print(gen.to_data(out length) + "\n");
    } // end serialize

    protected void daemonize_server () {
        int pid;
        int lockfp;
        string str;

        if (getppid() == 1) {
            return;
        }
        pid = fork();
        if (pid < 0) {
            exit(1);
        }
        if (pid > 0) {
            exit(0);
        }

        /* Try to become root, but ignore if we can't */
        setuid((uid_t) 0);

        setsid();
        for (pid = getdtablesize(); pid>=0; --pid) {
            close(pid);
        }
        pid = open("/dev/null", O_RDWR); dup(pid); dup(pid);
        umask((mode_t) 022);
        lockfp = Posix.open(lock_file, Posix.O_RDWR | Posix.O_CREAT, 0644);
        if (lockfp < 0) {
            syslog(LOG_ERR, "Could not serialize PID to lock file");
            exit(1);
        }
/*
        Flock fl = Flock();
        fl.l_type = Posix.F_WRLCK;
        int fcntl_return = Posix.fcntl (lockfp, Posix.F_SETLK, fl);
        if (fcntl_return == -1) {
            syslog(LOG_ERR, "Could not create lock, bailing out");
            exit(0);
        }
*/
        str = "%d\n".printf(getpid());
        write(lockfp, str, strlen(str));
        close(lockfp);

        /* Signal handling */
        Posix.signal(Posix.SIGCHLD, signal_handler);
        Posix.signal(Posix.SIGTSTP, signal_handler);
        Posix.signal(Posix.SIGTTOU, signal_handler);
        Posix.signal(Posix.SIGTTIN, signal_handler);
        Posix.signal(Posix.SIGHUP , signal_handler);
        Posix.signal(Posix.SIGTERM, signal_handler);
    } // end daemonize_server

} // end class Vded

