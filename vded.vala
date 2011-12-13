/**
 * VDED
 * https://github.com/jbuchbinder/vded
 *
 * vim: tabstop=4:softtabstop=4:shiftwidth=4:expandtab
 */

using GLib;
using Posix;
using Soup;

class Vded {

    public class Vector : Object {
        public string host { get; set construct; }
        public string name { get; set construct; }
        public Gee.HashMap<long, string> values { get; set construct; }
        public uint64 latest_value { get; set construct; }

        public void init_values() {
            values = new Gee.HashMap<long, string>();
        }

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
    protected Gee.HashMap<string, Vded.Vector> vectors;
    protected Soup.Server rest_server;

    public static void main (string[] args) {
        // Initialize syslog
        Posix.openlog( "vded", LOG_CONS | LOG_PID, LOG_LOCAL0 ); 

        Vded c = new Vded();
        c.init(args);
    } // end main

    Vded() {
        vectors = new Gee.HashMap<string, Vded.Vector>();
    }

    ~Vded() {
        // Handle gracefully shutting down the REST server.
        if (rest_server != null) {
            rest_server.quit();
        }
    } // end destructor

    public string get_key_name(string host, string vector_name) {
        return host + "/" + vector_name;
    } // end keyname

    public void init (string[] args) {
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
            print("root path requested!\n");
        } else if (path == "/favicon.ico") {
            // TODO: serve up a friendly favicon?
        } else if (path == "/submit") {
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
            print("key = %s\n", key);

            Vded.Vector v = null;
            bool create_new = false;
            if (!vectors.has_key(key)) {
                print("Need to create new key %s\n", key);
                create_new = true;
                v = new Vded.Vector();
                v.init_values();
                v.host = hostname;
                v.name = vector_name;
            } else {
                v = vectors.get(key);
            }

            // Add value to list of vectors
            v.add_value(long.parse(ts), value);

            if (create_new) {
                print("Storing new key\n");
                vectors.set(key, v);
            }

            build_return_values(msg, v);

        } else {
            print("path = %s\n", path);
            msg.set_status(404);
            msg.set_response("text/plain", Soup.MemoryUse.COPY, "Path not found.".data);
        }
    } // end rest_callback

    public void build_return_values (Soup.Message msg, Vded.Vector vector) {
        if (vector.values.size == 0) {
            rest_error(msg, 3, vector.to_string() + " has no values");
            return;
        }

        uint64 last_diff = 0;

        // TODO : sort set, etc
        if (vector.values.size == 1) {
            // Only one, return just that, no calculation
            last_diff = vector.latest_value;
        } else {
            // Need to determine differences
            long[] keys = vector.values.keys.to_array();
            long max1 = keys[keys.length - 1];
            long max2 = keys[keys.length - 2];

            last_diff = uint64.parse(vector.values.get(max1)) - uint64.parse(vector.values.get(max2));
        }

        string response = "{\"last_diff\":%s}".printf(last_diff.to_string());
        msg.set_status(200);
        msg.set_response("application/json", Soup.MemoryUse.COPY, response.data);
    } // end build_return_values

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

} // end class Vded

