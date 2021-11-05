# vastai_exporter

For [Vast.ai](https://vast.ai) hosts.

Prometheus exporter reporting data from your Vast.ai account:

- Stats of your machines: reliability, inet speed, number of client jobs running.
- Stats of your own instances: on-demand and default.
- Paid and pending balance of your account.
- Your on-demand and bid prices. 
- Stats on other hosts' offers with the same GPU as yours.

In addition to per-account Prometheus metrics (url: `/metrics`), the exporter provides the following data:

- Global stats over all types of GPUs (url: `/metrics/global`).
- List of offers available on Vast.ai (url: `/offers`).
- List of machines available on Vast.ai (url: `/machines`).

_NOTE: This is a work in progress. Output format is subject to change._

### Usage

```
docker run -d --restart always -p 8622:8622 sergeycheperis/ethereum-exporter --key=VASTKEY ARGS...
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

_NOTE: This example is annotated and edited for readability. It is fake and not a representation of any real account._


```
### Info on your machines

# HELP vastai_machine_info Machine info
vastai_machine_info{gpu_name="RTX 3080",hostname="rig1.local",machine_id="2100"} 1
vastai_machine_info{gpu_name="RTX 3080",hostname="rig2.local",machine_id="3100"} 1

# HELP vastai_machine_gpu_count Number of GPUs
vastai_machine_gpu_count{machine_id="2100"} 2
vastai_machine_gpu_count{machine_id="3100"} 2

# HELP vastai_machine_inet_bps Measured internet speed, download or upload (direction = 'up'/'down')
vastai_machine_inet_bps{direction="down",id="2100"} 4.397e+08
vastai_machine_inet_bps{direction="down",id="3100"} 4.831e+08

# HELP vastai_machine_is_listed Is machine listed (1) or not (0)
vastai_machine_is_listed{machine_id="2100"} 1
vastai_machine_is_listed{machine_id="3100"} 1

# HELP vastai_machine_is_online Is machine online (1) or not (0)
vastai_machine_is_online{machine_id="2100"} 1
vastai_machine_is_online{machine_id="3100"} 1

# HELP vastai_machine_is_verified Is machine verified (1) or not (0)
vastai_machine_is_verified{machine_id="2100"} 1
vastai_machine_is_verified{machine_id="3100"} 1

# HELP vastai_machine_ondemand_price_per_gpu_dollars Machine on-demand price per GPU/hour
vastai_machine_ondemand_price_per_gpu_dollars{machine_id="2100"} 0.7
vastai_machine_ondemand_price_per_gpu_dollars{machine_id="3100"} 0.7

# HELP vastai_machine_reliability Reliability indicator (0.0-1.0)
vastai_machine_reliability{machine_id="2100"} 0.9930448
vastai_machine_reliability{machine_id="3100"} 0.9925481

# HELP vastai_machine_rentals_count Count of current rentals (rental_type = 'ondemand'/'bid'/'default', rental_status = 'running'/'stopped')
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


### Info on your instances (these include default jobs and jobs started by you)

# HELP vastai_instance_info Instance info
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="1414830",machine_id="2100",rental_type="default"} 1
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="1414831",machine_id="2100",rental_type="default"} 1
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="922837",machine_id="3100",rental_type="default"} 1
vastai_instance_info{docker_image="example/ethminer",gpu_name="RTX 3080",instance_id="922838",machine_id="3100",rental_type="default"} 1

# HELP vastai_instance_gpu_count Number of GPUs assigned to this instance
vastai_instance_gpu_count{instance_id="1414830",machine_id="2100",rental_type="default"} 1
vastai_instance_gpu_count{instance_id="1414831",machine_id="2100",rental_type="default"} 1
vastai_instance_gpu_count{instance_id="922837",machine_id="3100",rental_type="default"} 1
vastai_instance_gpu_count{instance_id="922838",machine_id="3100",rental_type="default"} 1

# HELP vastai_instance_gpu_fraction Number of GPUs assigned to this instance divided by total number of GPUs on the host
vastai_instance_gpu_fraction{instance_id="1414830",machine_id="2100",rental_type="default"} 0.5
vastai_instance_gpu_fraction{instance_id="1414831",machine_id="2100",rental_type="default"} 0.5
vastai_instance_gpu_fraction{instance_id="922837",machine_id="3100",rental_type="default"} 0.5
vastai_instance_gpu_fraction{instance_id="922838",machine_id="3100",rental_type="default"} 0.5

# HELP vastai_instance_is_running Is instance running (1) or stopped/outbid/initializing (0)
vastai_instance_is_running{instance_id="1414830",machine_id="2100",rental_type="default"} 0
vastai_instance_is_running{instance_id="1414831",machine_id="2100",rental_type="default"} 0
vastai_instance_is_running{instance_id="922837",machine_id="3100",rental_type="default"} 0
vastai_instance_is_running{instance_id="922838",machine_id="3100",rental_type="default"} 0

# HELP vastai_instance_min_bid_per_gpu_dollars Min bid to outbid this instance per GPU/hour (makes sense if rental_type = 'default'/'bid')
vastai_instance_min_bid_per_gpu_dollars{instance_id="1414830",machine_id="2100",rental_type="default"} 0.2884722
vastai_instance_min_bid_per_gpu_dollars{instance_id="1414831",machine_id="2100",rental_type="default"} 0.2884722
vastai_instance_min_bid_per_gpu_dollars{instance_id="922837",machine_id="3100",rental_type="default"} 0.2867361
vastai_instance_min_bid_per_gpu_dollars{instance_id="922838",machine_id="3100",rental_type="default"} 0.2969444

# HELP vastai_instance_my_bid_per_gpu_dollars My bid on this instance per GPU/hour
vastai_instance_my_bid_per_gpu_dollars{instance_id="1414830",machine_id="2100",rental_type="default"} 0.2
vastai_instance_my_bid_per_gpu_dollars{instance_id="1414831",machine_id="2100",rental_type="default"} 0.2
vastai_instance_my_bid_per_gpu_dollars{instance_id="922837",machine_id="3100",rental_type="default"} 0.2
vastai_instance_my_bid_per_gpu_dollars{instance_id="922838",machine_id="3100",rental_type="default"} 0.2

# HELP vastai_instance_start_timestamp Unix timestamp when instance was started
vastai_instance_start_timestamp{instance_id="1414830",machine_id="2100",rental_type="default"} 1.63036361926469e+09
vastai_instance_start_timestamp{instance_id="1414831",machine_id="2100",rental_type="default"} 1.63036361927396e+09
vastai_instance_start_timestamp{instance_id="922837",machine_id="3100",rental_type="default"} 1.6225778577921e+09
vastai_instance_start_timestamp{instance_id="922838",machine_id="3100",rental_type="default"} 1.62257785780379e+09


### Your payout stats

# HELP vastai_paid_out_dollars All-time paid out amount (minus service fees)
vastai_paid_out_dollars 303.34

# HELP vastai_pending_payout_dollars Pending payout (minus service fees)
vastai_pending_payout_dollars 28.23

# HELP last_payout_time Unix timestamp of last completed payout
last_payout_time 1628284623.45397


### Overall GPU offer stats (shows stats on GPUs same as yours but offered by other hosts)

# HELP vastai_gpu_count Number of GPUs offered on site (excluding yours)
vastai_gpu_count{gpu_name="RTX 3080",rented="no",verified="no"} 12
vastai_gpu_count{gpu_name="RTX 3080",rented="no",verified="yes"} 1
vastai_gpu_count{gpu_name="RTX 3080",rented="yes",verified="no"} 90
vastai_gpu_count{gpu_name="RTX 3080",rented="yes",verified="yes"} 120

# HELP vastai_ondemand_price_median_dollars Median on-demand price among same-type GPUs
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="any",verified="any"} 0.38
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="any",verified="no"} 0.36
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="any",verified="yes"} 0.38
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="no",verified="any"} 0.4
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="no",verified="no"} 0.4
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="no",verified="yes"} 1.1
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="yes",verified="any"} 0.38
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="yes",verified="no"} 0.36
vastai_ondemand_price_median_dollars{gpu_name="RTX 3080",rented="yes",verified="yes"} 0.38

# HELP vastai_ondemand_price_10th_percentile_dollars 10th percentile of on-demand prices among same-type GPUs
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="any",verified="any"} 0.26
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="any",verified="no"} 0.26
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="any",verified="yes"} 0.32
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="no",verified="any"} 0.285
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="no",verified="no"} 0.285
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="no",verified="yes"} 1.1
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="yes",verified="any"} 0.26
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="yes",verified="no"} 0.26
vastai_ondemand_price_10th_percentile_dollars{gpu_name="RTX 3080",rented="yes",verified="yes"} 0.32

# HELP vastai_ondemand_price_90th_percentile_dollars 90th percentile of on-demand prices among same-type GPUs
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="any",verified="any"} 0.5
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="any",verified="no"} 0.5
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="any",verified="yes"} 0.625
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="no",verified="any"} 0.49
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="no",verified="no"} 0.49
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="no",verified="yes"} 1.1
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="yes",verified="any"} 0.5
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="yes",verified="no"} 0.5
vastai_ondemand_price_90th_percentile_dollars{gpu_name="RTX 3080",rented="yes",verified="yes"} 0.65
```
