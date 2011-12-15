/**
 * VDED
 * https://github.com/jbuchbinder/vded
 *
 * vim: tabstop=4:softtabstop=4:shiftwidth=4:expandtab
 */

using GLib;
using Posix;

class Gmetric : GLib.Object {

    public string ganglia_host { construct set; get; }
    public int ganglia_port { construct set; get; }

    Gmetric ( string ganglia_host, int ganglia_port ) {
        this.ganglia_host = ganglia_host;
        this.ganglia_port = ganglia_port;
    } // end Gmetric

    protected string create_metric ( ) {
        // TODO, create metric text from object definition
    } // end create_metric

    public async void send_metric ( ) {
        try {
            var resolver = Resolver.get_default ();
            var addresses = yield resolver.lookup_by_name_async (ganglia_host);
            var address = addresses.nth_data (0);
            var addr = new InetSocketAddress(address, (uint16) ganglia_port);
            SocketClient sc = new SocketClient();
            sc.family = SocketFamily.IPV4;
            sc.protocol = SocketProtocol.UDP;
            sc.timeout = 5000;
            var conn = yield sc.connect_async(addr);
            conn.socket.set_blocking(false);
            var output = new DataOutputStream(conn.output_stream);
        } catch (Error e) {
            syslog(LOG_ERR, e.message);
        }
    } // end send_metric

} // end class Gmetric

