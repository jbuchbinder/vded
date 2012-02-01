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

class VdeClient {

    public bool debug = false;

    public string vded_host;
    public int vded_port = 48333;
    public string metric_name;
    public string metric_value;

    public static void main (string[] args) {
        VdeClient c = new VdeClient();
        c.init(args);
    } // end main

    public void syntax() {
        print("VDE CLIENT ( https://github.com/jbuchbinder/vded )\n" +
            "\n" +
            "Flags:\n" +
            "\t-v             verbose\n" +
            "\t-h HOST        specify vded hostname\n" +
            "\t-p PORT        specify vded port\n" +
            "\t-n METRIC      specify metric name\n" +
            "\t-v VALUE       specify metric value\n" +
            "\n");
        syslog(LOG_ERR, "Syntax error encountered parsing command line arguments.");
        exit(1);
    } // end syntax

    public void init (string[] args) {
        // Parse args
        for (int i=1; i<args.length; i++) {
            if (args[i] == "-v") {
                debug = true;
            } else if (args[i] == "-h") {
                if (args.length >= i+1) {
                    vded_host = args[++i];
                } else {
                    syntax();
                }
            } else if (args[i] == "-p") {
                if (args.length >= i+1) {
                    vded_port = int.parse(args[++i]);
                } else {
                    syntax();
                }
            } else if (args[i] == "-n") {
                if (args.length >= i+1) {
                    metric_name = args[++i];
                } else {
                    syntax();
                }
            } else if (args[i] == "-v") {
                if (args.length >= i+1) {
                    metric_value = args[++i];
                } else {
                    syntax();
                }
            } else {
                // Handle unknown arguments
                syntax();
            }
        }
    }

} // end class VdeClient

