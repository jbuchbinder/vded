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
        VALUE_UNKNOWN         = 0,
        VALUE_STRING          = 1,
        VALUE_UNSIGNED_SHORT  = 2,
        VALUE_SHORT           = 3,
        VALUE_UNSIGNED_INT    = 4,
        VALUE_INT             = 5,
        VALUE_FLOAT           = 6,
        VALUE_DOUBLE          = 7;

        public string to_string ( ) {
            switch (this) {
                case VALUE_STRING          : return "string";
                case VALUE_UNSIGNED_SHORT  : return "uint16";
                case VALUE_SHORT           : return "int16";
                case VALUE_UNSIGNED_INT    : return "uint32";
                case VALUE_INT             : return "int32";
                case VALUE_FLOAT           : return "float";
                case VALUE_DOUBLE          : return "double";
                case VALUE_UNKNOWN         : return "unknown";
            }
            return "string";
        } // end to_string

        public string to_format_string ( ) {
            switch (this) {
                case VALUE_STRING          : return "%s";
                case VALUE_UNSIGNED_SHORT  : return "%uh";
                case VALUE_SHORT           : return "%h";
                case VALUE_UNSIGNED_INT    : return "%u";
                case VALUE_INT             : return "%d";
                case VALUE_FLOAT           : return "%f";
                case VALUE_DOUBLE          : return "%lf";
                case VALUE_UNKNOWN         : return "";
            }
            return "";
        } // end to_format_string

    } // end ValueType

    public string ganglia_host { construct set; get; }
    public int ganglia_port { construct set; get; }
    public string? ganglia_spoof { construct set; get; }

    public static string SPOOF_HOST = "SPOOF_HOST";

    Gmetric ( string ganglia_host, int ganglia_port, string? ganglia_spoof ) {
        this.ganglia_host = ganglia_host;
        this.ganglia_port = ganglia_port;
        if (ganglia_spoof != null)
            this.ganglia_spoof = ganglia_spoof;
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
            ByteArray meta = writemeta(host, name, type.to_string(), units, (int) slope, tmax, dmax, ganglia_spoof);
            foreach ( uint8 b_meta in meta.data ) {
                output.put_byte(b_meta);
            }
            ByteArray metric = writevalue( host, name, type, value, ganglia_spoof );
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
        out[0] = (uint8) ((i << 24) & 0xff);
        out[1] = (uint8) ((i << 16) & 0xff);
        out[2] = (uint8) ((i >>  8) & 0xff);
        out[3] = (uint8) ( i        & 0xff);
        return out;
    } // end int_to_bytes

    private uint8[] double_to_bytes(double d) {
        uint8[8] out = { };
        // TODO: FIXME:
        // I believe this should be double -> long -> 8 bytes
        return out;
    } // end double_to_bytes

    private uint8[] bool_to_bytes(bool b) {
        uint8[1] out = { };
        out[0] = (uint8) ( b ? 1 : 0 );
        return out;
    } // end bool_to_bytes

    private void writeXDRString(ref ByteArray b, string s) {
        b.append(int_to_bytes(s.length));
        b.append(string_to_bytes(s));
        int offset = s.length % 4;
        if (offset != 0) {
            for (int i = offset; i < 4; ++i) {
                b.append({ (uint8) 0 });
            }
        }
    } // end writeXDRString

    private ByteArray writevalue(string host, string name, ValueType type, string val, string? spoof) {
        ByteArray b = new ByteArray();
        b.append(int_to_bytes(128 + type));
        writeXDRString(ref b, host);
        writeXDRString(ref b, name);
        b.append(bool_to_bytes(spoof != null));
        writeXDRString(ref b, type.to_format_string());
        switch (type) {
            case ValueType.VALUE_UNSIGNED_INT    :
                b.append(int_to_bytes(int.parse(val))); break;

            case ValueType.VALUE_UNSIGNED_SHORT  :
            case ValueType.VALUE_SHORT           :
            case ValueType.VALUE_INT             :
            case ValueType.VALUE_FLOAT           :
            case ValueType.VALUE_DOUBLE          :
            case ValueType.VALUE_STRING          :
            case ValueType.VALUE_UNKNOWN         :
                writeXDRString(ref b, val); break;
        }
        return b;
    } // end writevalue

    private ByteArray writemeta(string host, string name, string type, string units, int slope, int tmax, int dmax, string? spoof) {
        ByteArray b = new ByteArray();
        b.append(int_to_bytes(128));  // gmetadata_full
        writeXDRString(ref b, host);
        writeXDRString(ref b, name);
        b.append(bool_to_bytes(spoof != null)); // spoof
        
        writeXDRString(ref b, type);
        writeXDRString(ref b, name);
        writeXDRString(ref b, units);
        b.append(int_to_bytes(slope));
        b.append(int_to_bytes(tmax));
        b.append(int_to_bytes(dmax));

        if (spoof == null) {
            b.append(int_to_bytes(0));
        } else {
            b.append(int_to_bytes(1));
            writeXDRString(ref b, SPOOF_HOST);
            writeXDRString(ref b, spoof);
         }

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

