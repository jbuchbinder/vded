/**
 * VDED
 * https://github.com/jbuchbinder/vded
 *
 * vim: tabstop=4:softtabstop=4:shiftwidth=4:expandtab
 */

using GLib;
using Posix;

class Gmetric : GLib.Object {

    public enum Slope {
        ZERO         = 0,
        POSITIVE     = 1,
        NEGATIVE     = 2,
        BOTH         = 3,
        UNSPECIFIED  = 4
    }

    public enum ValueType {
        VALUE_STRING          = 0,
        VALUE_UNSIGNED_SHORT  = 1,
        VALUE_SHORT           = 2,
        VALUE_UNSIGNED_INT    = 3,
        VALUE_INT             = 4,
        VALUE_FLOAT           = 5,
        VALUE_DOUBLE          = 6,
        VALUE_TIMESTAMP       = 7;

        public string to_string ( ) {
            switch (this) {
                case VALUE_STRING          : return "string";
                case VALUE_UNSIGNED_SHORT  : return "uint16";
                case VALUE_SHORT           : return "int16";
                case VALUE_UNSIGNED_INT    : return "uint32";
                case VALUE_INT             : return "int32";
                case VALUE_FLOAT           : return "float";
                case VALUE_DOUBLE          : return "double";
                case VALUE_TIMESTAMP       : return "timestamp";
            }
            return "string";
        } // end to_string

    } // end ValueType

    public string ganglia_host { construct set; get; }
    public int ganglia_port { construct set; get; }

    Gmetric ( string ganglia_host, int ganglia_port ) {
        this.ganglia_host = ganglia_host;
        this.ganglia_port = ganglia_port;
    } // end Gmetric

    public async void send_metric ( string host, string name, string value, string units, ValueType type, Slope slope, int tmax, int dmax ) {
        try {
            var resolver = Resolver.get_default ();
            var addresses = yield resolver.lookup_by_name_async (ganglia_host);
            var address = addresses.nth_data (0);
            InetSocketAddress addr = new InetSocketAddress(address, (uint16) ganglia_port);
            bool is_multicast = addr.get_address().get_is_multicast();
            SocketClient sc = new SocketClient();
            sc.family = SocketFamily.IPV4;
            sc.protocol = SocketProtocol.UDP;
            sc.type = SocketType.STREAM;
            sc.timeout = 5;
            var conn = yield sc.connect_async(addr);
            conn.socket.set_blocking(false);
            var output = new DataOutputStream(conn.output_stream);
            // Push meta, then metric
            ByteArray meta = writemeta(host, name, type.to_string(), units, (int) slope, tmax, dmax);
            foreach ( uint8 b_meta in meta.data ) {
                output.put_byte(b_meta);
            }
            ByteArray metric = writevalue( host, name, value );
            foreach ( uint8 b_metric in metric.data ) {
                output.put_byte(b_metric);
            }
        } catch (Error e) {
            syslog(LOG_ERR, e.message);
        }
    } // end send_metric

    //-------------------------------------------------------------------
    //     PRIVATE METHODS FOR CREATING METRIC PACKETS
    //-------------------------------------------------------------------

    private uint8[] string_to_bytes(string s) {
        uint8[] out = { };
        if (s == null) { return out; }
        for (int i=0; i<s.length; i++) {
            out[i] = (uint8) s[i];
        }
        return out;
    } // end string_to_bytes

    private uint8[] int_to_bytes(int i) {
        uint8[4] out = { };
        out[0] = (uint8) i << 24;
        out[1] = (uint8) i << 16;
        out[2] = (uint8) i >> 8;
        out[3] = (uint8) i & 0xff;
        return out;
    } // end int_to_bytes

    private void writeXDRString(ByteArray b, string s) {
        b.append(int_to_bytes(s.length));
        b.append(string_to_bytes(s));
        int offset = s.length % 4;
        if (offset != 0) {
            for (int i = offset; i < 4; ++i) {
                b.append({ (uint8) 0 });
            }
        }
    } // end writeXDRString

    private ByteArray writevalue(string host, string name, string val) {
        ByteArray b = new ByteArray();
        b.append(int_to_bytes(128+5));  // string
        writeXDRString(b, host);
        writeXDRString(b, name);
        b.append(int_to_bytes(0));
        writeXDRString(b, "%s");
        writeXDRString(b, val);
        return b;
    } // end writevalue

    private ByteArray writemeta(string host, string name, string type, string units, int slope, int tmax, int dmax) {
        ByteArray b = new ByteArray();
        b.append(int_to_bytes(128));  // gmetadata_full
        writeXDRString(b, host);
        writeXDRString(b, name);
        b.append(int_to_bytes(0));
        
        writeXDRString(b, type);
        writeXDRString(b, name);
        writeXDRString(b, units);
        b.append(int_to_bytes(slope));
        b.append(int_to_bytes(tmax));
        b.append(int_to_bytes(dmax));
        b.append(int_to_bytes(0));
        
        // to add extra metadata it's something like this
        // assuming extradata is hashmap , then:
        //
        // write extradata.size();
        // foreach key,value in "extradata"
        //   writeXDRString(dos, keyey)
        //   writeXDRString(dos, value)
        return b;
    } // end writemeta

    private static uint8[] HEXCHARS = {
        (uint8)'0', (uint8)'1', (uint8)'2', (uint8)'3',
        (uint8)'4', (uint8)'5', (uint8)'6', (uint8)'7',
        (uint8)'8', (uint8)'9', (uint8)'a', (uint8)'b',
        (uint8)'c', (uint8)'d', (uint8)'e', (uint8)'f'
    };

    private static string bytes2hex (uint8[] raw) {
        int pos = 0;
        uint8[] hex = { };
        for (int i = 0; i < raw.length; ++i) {
            int v = raw[i] & 0xFF;
            hex[pos++] = HEXCHARS[v >> 4];
            hex[pos++] = HEXCHARS[v & 0xF];
        }
        return (string) hex;
    }

} // end class Gmetric

