# vastai_exporter

For [Vast.ai](https://vast.ai) hosts.

Prometheus exporter reporting data from your Vast.ai account:

- Stats of your machines: reliability, inet speed, number of client jobs running.
- Stats of your own instances: on-demand and default.
- Paid and pending balance of your account.
- Your on-demand and bid prices. 
- On-demand price range of machines with the same GPU as yours.

_NOTE: This is a work in progress. Output format is subject to change._

### Usage

```
docker run -d --restart always -p 8622:8622 sergeycheperis/ethereum-exporter --api-key=VASTKEY ARGS...
```
Replace _VASTKEY_ with your Vast.ai API key. To test, open http://localhost:8622. If does not work, check container output with `docker logs`.


### Optional args

```
--listen=IP:PORT
    Address to listen on (default 0.0.0.0:8622).

--update-interval=
    How often to query Vast.ai for updates (default 1m).
```

### Example output

```
# HELP vastai_instance_gpu_count Number of GPUs assigned to this instance
# TYPE vastai_instance_gpu_count gauge
vastai_instance_gpu_count{instance_id="1414830",machine_id="2100",rental_type="default"} 1
vastai_instance_gpu_count{instance_id="1414831",machine_id="2100",rental_type="default"} 1
vastai_instance_gpu_count{instance_id="922837",machine_id="3100",rental_type="default"} 1
vastai_instance_gpu_count{instance_id="922838",machine_id="3100",rental_type="default"} 1

# HELP vastai_instance_gpu_fraction Number of GPUs assigned to this instance divided by total number of GPUs on the host
# TYPE vastai_instance_gpu_fraction gauge
vastai_instance_gpu_fraction{instance_id="1414830",machine_id="2100",rental_type="default"} 0.5
vastai_instance_gpu_fraction{instance_id="1414831",machine_id="2100",rental_type="default"} 0.5
vastai_instance_gpu_fraction{instance_id="922837",machine_id="3100",rental_type="default"} 0.5
vastai_instance_gpu_fraction{instance_id="922838",machine_id="3100",rental_type="default"} 0.5

# HELP vastai_instance_info Instance info
# TYPE vastai_instance_info gauge
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="1414830",machine_id="2100",rental_type="default"} 1
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="1414831",machine_id="2100",rental_type="default"} 1
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="922837",machine_id="3100",rental_type="default"} 1
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="922838",machine_id="3100",rental_type="default"} 1

# HELP vastai_instance_is_running Is instance running (1) or stopped/outbid/initializing (0)
# TYPE vastai_instance_is_running gauge
vastai_instance_is_running{instance_id="1414830",machine_id="2100",rental_type="default"} 0
vastai_instance_is_running{instance_id="1414831",machine_id="2100",rental_type="default"} 0
vastai_instance_is_running{instance_id="922837",machine_id="3100",rental_type="default"} 0
vastai_instance_is_running{instance_id="922838",machine_id="3100",rental_type="default"} 0

# HELP vastai_instance_min_bid_per_gpu_dollars Min bid to outbid this instance per GPU/hour (makes sense if rental_type = 'default'/'bid')
# TYPE vastai_instance_min_bid_per_gpu_dollars gauge
vastai_instance_min_bid_per_gpu_dollars{instance_id="1414830",machine_id="2100",rental_type="default"} 0.2884722
vastai_instance_min_bid_per_gpu_dollars{instance_id="1414831",machine_id="2100",rental_type="default"} 0.2884722
vastai_instance_min_bid_per_gpu_dollars{instance_id="922837",machine_id="3100",rental_type="default"} 0.2867361
vastai_instance_min_bid_per_gpu_dollars{instance_id="922838",machine_id="3100",rental_type="default"} 0.2969444

# HELP vastai_instance_my_bid_per_gpu_dollars My bid on this instance per GPU/hour
# TYPE vastai_instance_my_bid_per_gpu_dollars gauge
vastai_instance_my_bid_per_gpu_dollars{instance_id="1414830",machine_id="2100",rental_type="default"} 0.2
vastai_instance_my_bid_per_gpu_dollars{instance_id="1414831",machine_id="2100",rental_type="default"} 0.2
vastai_instance_my_bid_per_gpu_dollars{instance_id="922837",machine_id="3100",rental_type="default"} 0.2
vastai_instance_my_bid_per_gpu_dollars{instance_id="922838",machine_id="3100",rental_type="default"} 0.2

# HELP vastai_instance_start_timestamp Unix timestamp when instance was started
# TYPE vastai_instance_start_timestamp gauge
vastai_instance_start_timestamp{instance_id="1414830",machine_id="2100",rental_type="default"} 1.63036361926469e+09
vastai_instance_start_timestamp{instance_id="1414831",machine_id="2100",rental_type="default"} 1.63036361927396e+09
vastai_instance_start_timestamp{instance_id="922837",machine_id="3100",rental_type="default"} 1.6225778577921e+09
vastai_instance_start_timestamp{instance_id="922838",machine_id="3100",rental_type="default"} 1.62257785780379e+09

# HELP vastai_machine_gpu_count Number of GPUs
# TYPE vastai_machine_gpu_count gauge
vastai_machine_gpu_count{machine_id="2100"} 2
vastai_machine_gpu_count{machine_id="3100"} 2

# HELP vastai_machine_inet_bps Measured internet speed, download or upload (direction = 'up'/'down')
# TYPE vastai_machine_inet_bps gauge
vastai_machine_inet_bps{direction="down",id="2100"} 4.397e+08
vastai_machine_inet_bps{direction="down",id="3100"} 4.831e+08

# HELP vastai_machine_info Machine info
# TYPE vastai_machine_info gauge
vastai_machine_info{gpu_name="RTX 3080",hostname="rig1.local",machine_id="2100"} 1
vastai_machine_info{gpu_name="RTX 3080",hostname="rig2.local",machine_id="3100"} 1

# HELP vastai_machine_is_listed Is machine listed (1) or not (0)
# TYPE vastai_machine_is_listed gauge
vastai_machine_is_listed{machine_id="2100"} 1
vastai_machine_is_listed{machine_id="3100"} 1

# HELP vastai_machine_is_online Is machine online (1) or not (0)
# TYPE vastai_machine_is_online gauge
vastai_machine_is_online{machine_id="2100"} 1
vastai_machine_is_online{machine_id="3100"} 1

# HELP vastai_machine_is_verified Is machine verified (1) or not (0)
# TYPE vastai_machine_is_verified gauge
vastai_machine_is_verified{machine_id="2100"} 1
vastai_machine_is_verified{machine_id="3100"} 1

# HELP vastai_machine_ondemand_price_per_gpu_dollars Machine on-demand price per GPU/hour
# TYPE vastai_machine_ondemand_price_per_gpu_dollars gauge
vastai_machine_ondemand_price_per_gpu_dollars{machine_id="2100"} 0.7
vastai_machine_ondemand_price_per_gpu_dollars{machine_id="3100"} 0.7

# HELP vastai_machine_reliability Reliability indicator (0.0-1.0)
# TYPE vastai_machine_reliability gauge
vastai_machine_reliability{machine_id="2100"} 0.9930448
vastai_machine_reliability{machine_id="3100"} 0.9925481

# HELP vastai_machine_rentals_count Count of current rentals (rental_type = 'ondemand'/'bid'/'default', rental_status = 'running'/'stopped')
# TYPE vastai_machine_rentals_count gauge
vastai_machine_rentals_count{machine_id="2100",rental_status="running",rental_type="bid"} 1
vastai_machine_rentals_count{machine_id="2100",rental_status="running"} 0
vastai_machine_rentals_count{machine_id="2100",rental_status="running",rental_type="my"} 0
vastai_machine_rentals_count{machine_id="2100",rental_status="running",rental_type="ondemand"} 2
vastai_machine_rentals_count{machine_id="2100",rental_status="stopped",rental_type="bid"} 6
vastai_machine_rentals_count{machine_id="2100",rental_status="stopped"} 4
vastai_machine_rentals_count{machine_id="2100",rental_status="stopped",rental_type="my"} 0
vastai_machine_rentals_count{machine_id="2100",rental_status="stopped",rental_type="ondemand"} 15
vastai_machine_rentals_count{machine_id="3100",rental_status="running",rental_type="bid"} 1
vastai_machine_rentals_count{machine_id="3100",rental_status="running"} 0
vastai_machine_rentals_count{machine_id="3100",rental_status="running",rental_type="my"} 0
vastai_machine_rentals_count{machine_id="3100",rental_status="running",rental_type="ondemand"} 2
vastai_machine_rentals_count{machine_id="3100",rental_status="stopped",rental_type="bid"} 4
vastai_machine_rentals_count{machine_id="3100",rental_status="stopped"} 4
vastai_machine_rentals_count{machine_id="3100",rental_status="stopped",rental_type="my"} 0
vastai_machine_rentals_count{machine_id="3100",rental_status="stopped",rental_type="ondemand"} 6

# HELP vastai_ondemand_price_10th_percentile_dollars 10th percentile of on-demand prices among verified GPUs
# TYPE vastai_ondemand_price_10th_percentile_dollars gauge
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080"} 0.3

# HELP vastai_ondemand_price_90th_percentile_dollars 90th percentile of on-demand prices among verified GPUs
# TYPE vastai_ondemand_price_90th_percentile_dollars gauge
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080"} 0.4

# HELP vastai_ondemand_price_median_dollars Median on-demand price among verified GPUs
# TYPE vastai_ondemand_price_median_dollars gauge
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080"} 0.35

# HELP vastai_paid_out_dollars All-time paid out amount (minus service fees)
# TYPE vastai_paid_out_dollars gauge
vastai_paid_out_dollars 303.34

# HELP vastai_pending_payout_dollars Pending payout (minus service fees)
# TYPE vastai_pending_payout_dollars gauge
vastai_pending_payout_dollars 28.23
```

_NOTE: This output is fake and not a representation of any real account._
