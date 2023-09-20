# Kernel Parameters for Lower Latency System

To make Easegress good performance, some of the kernel parameters might need to be optimized. Here is a possible configuration for `/etc/sysctl.conf` and `/etc/security/limits.conf`.

## /etc/security/limits.conf

The values need to be aligned with actual memory.

```conf
*   soft    nproc         2067554
*   hard    nproc         2067554
``` 

## /etc/sysctl.conf

The values need to be aligned with actual memory, CPU, and network.

1. Increase range of ephemeral ports that can be used
    ```conf
    net.ipv4.ip_local_port_range = 1024 65535
    ```

2. Increase the max number of open files
    ```conf
    fs.file-max = 150000
    ```

3. Enable TCP window scaling (enabled by default). Refer to [Wikipedia: TCP_window_scale_option](https://en.wikipedia.org/wiki/TCP_window_scale_option)
    ```conf
    net.ipv4.tcp_window_scaling = 1
    ```

4. Turn off SYN-flood protections
    ```conf
    net.ipv4.tcp_syncookies = 0
    ```

5. Increase the number of packets that can be queued in the network card before being handed to the CPU
    ```conf
    net.core.netdev_max_backlog = 3240000
    ```

6. Max number of "backlogged sockets" (connection requests that can be queued for any given listening socket)
    ```conf
    net.core.somaxconn = 65535
    ```

7. TCP memory tuning. Refer to [Linux Administration](http://www.linux-admins.net/2010/09/linux-tcp-tuning.html)

    ```conf
    # Increase the default socket buffer read size (rmem_default) and write size (wmem_default)
    # *** Maybe recommended only for high-RAM servers? ***
    net.core.rmem_default=16777216
    net.core.wmem_default=16777216
    
    # Increase the max socket buffer size (optmem_max), max socket buffer read size (rmem_max), max     socket buffer write size (wmem_max)
    # 16MB per socket - which sounds like a lot, but will virtually never consume that much
    # rmem_max over-rides tcp_rmem param, wmem_max over-rides tcp_wmem param and optmem_max over-rides     tcp_mem param
    net.core.optmem_max=16777216
    net.core.rmem_max=16777216
    net.core.wmem_max=16777216
    
    # Configure the Min, Pressure, Max values (units are in page size)
    # Useful mostly for very high-traffic websites that have a lot of RAM
    # Consider that we already set the *_max values to 16777216
    # So you may eventually comment on these three lines
    
    net.ipv4.tcp_mem=16777216 16777216 16777216
    net.ipv4.tcp_wmem=4096 87380 16777216
    net.ipv4.tcp_rmem=4096 87380 16777216
    ```

8. Number of packets to keep in the backlog before the kernel starts dropping them
    ```conf
    net.ipv4.tcp_max_syn_backlog = 3240000
    ```

9. Only retry creating TCP connections 3 times. Minimize the time it takes for a connection attempt to fail
    ```conf
    net.ipv4.tcp_syn_retries=3
    net.ipv4.tcp_synack_retries=3
    net.ipv4.tcp_orphan_retries=3
    ```

10. How many retries TCP makes on data segments (default 15). Some guides suggest reducing this value
    ```conf
    net.ipv4.tcp_retries2 = 8
    ```

11. Increase max number of sockets allowed in TIME_WAIT
    ```conf
    net.ipv4.tcp_max_tw_buckets = 1440000
    ```

12. Keepalive Optimizations. By default, the keepalive routines wait for two hours (7200 secs) before sending the first keepalive probe, and then resend it every 75 seconds. If no ACK response is received for 9 consecutive times, the connection is marked as broken.

    ```conf
    # We would decrease the default values for tcp_keepalive_* params as follow:
    net.ipv4.tcp_keepalive_time = 600  # default 7200
    net.ipv4.tcp_keepalive_intvl = 10  # default 75
    net.ipv4.tcp_keepalive_probes = 9  # default 9
    ```

13. The TCP FIN timeout specifies the amount of time a port must be inactive before it can be reused for another connection.  The default is often 60 seconds, but can normally be safely reduced to 30 or even 15 seconds
 https://www.linode.com/docs/web-servers/nginx/configure-nginx-for-optimized-performance

    ```conf
    net.ipv4.tcp_fin_timeout = 7
    ```

14. Refer to this [Github post](https://github.com/ton31337/tools/wiki/tcp_slow_start_after_idle---tcp_no_metrics_save-performance)

    ```conf
    # Avoid falling back to slow start after a connection goes idle.
    net.ipv4.tcp_slow_start_after_idle = 0

    # Disable caching of TCP congestion state
    net.ipv4.tcp_no_metrics_save = 1
    ```

15. If listening service is too slow to accept new connections, reset them. Default state is FALSE. It means that if overflow occurred due to a burst, connection will recover. Enable this option only if you are really sure that listening daemon cannot be tuned to accept connections faster. Enabling this option can harm clients of your server. Refer to [Github Post](https://github.com/ton31337/tools/wiki/Is-net.ipv4.tcp_abort_on_overflow-good-or-not%3F)

    ```conf
    net.ipv4.tcp_abort_on_overflow=0
    ```



