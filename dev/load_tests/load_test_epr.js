import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { randomIntBetween } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';

const BASE = `${__ENV.TARGET_HOST}`;

export const options = {
  scenarios: {
    // 1a. Steady (Load) Test
    // steady_load_iters: {
    //   executor: 'shared-iterations',
    //   exec: 'default',
    //   vus: `${__ENV.VUS_NUMBER}`,
    //   iterations: `${__ENV.ITERATIONS_NUMBER}`,
    //   maxDuration: '20m',
    //   startTime: '0s',
    //   tags: { test_type: 'steady_iters' },
    // },
    // 1b. Steady (Load) Test
    steady_load_vus: {
      executor: 'constant-vus',
      exec: 'default',
      // startTime: '30m10s',
      vus: `${__ENV.VUS_NUMBER}`,
      duration: `${__ENV.DURATION_TIME}`,
      tags: { test_type: 'steady_vus' },
    },
    // 2. Stress Test - progressively increasing load
    // stress: {
    //   executor: 'ramping-vus',
    //   exec: 'stressTest',
    //   startTime: '5m10s',
    //   stages: [
    //     { duration: '2m', target: 10 },
    //     { duration: '3m', target: 60 },
    //     { duration: '2m', target: 30 },
    //     { duration: '2m', target: 0 }, // ramp down
    //   ],
    //   tags: { test_type: 'stress' },
    // },
    // // 3. Spike Test - sudden surge
    // spike: {
    //   executor: 'ramping-vus',
    //   exec: 'default',
    //   startTime: '12m10s',
    //   stages: [
    //     { duration: '10s', target: 50 },   // baseline
    //     { duration: '20s', target: 300 },  // sudden spike
    //     { duration: '2m', target: 300 },   // hold at peak
    //     { duration: '30s', target: 0 },    // ramp down
    //   ],
    //   tags: { test_type: 'spike' },
    // },
    // // 4. Soak (Endurance) Test - sustained high load
    // soak: {
    //   executor: 'constant-vus',
    //   exec: 'default',
    //   startTime: '15m10s',
    //   vus: 75,
    //   duration: '10m',
    //   tags: { test_type: 'soak' },
    // },
  },
  thresholds: {
    // 'http_req_duration{test_type:steady_iters}': ['p(95)<15000'],  // 95% of requests should be below 4000ms
    'http_req_duration{test_type:steady_vus}': ['p(95)<15000'], // 95% of requests should be below 4000ms
    // 'http_req_duration{test_type:stress}': ['p(95)<1000'], // Not run
    // 'http_req_duration{test_type:spike}': ['p(95)<1500'],  // Not run
    // 'http_req_duration{test_type:soak}': ['p(95)<800'],    // Not run
  },
}

const packages = [
  '/package/apm/8.14.3/',
  '/package/apm/8.6.2/',
  '/package/aws/1.16.4/',
  '/package/beaconing/1.3.1/',
  '/package/zoom/1.2.1/',
  '/package/elastic_agent/2.6.7/',
  '/package/elastic_agent/2.6.3/',
  '/package/elastic_agent/2.5.0/',
  '/package/elastic_agent/1.10.0/',
  '/package/endpoint/9.2.0/',
  '/package/endpoint/9.1.0/',
  '/package/endpoint/9.0.2/',
  '/package/endpoint/8.19.0/',
  '/package/endpoint/8.18.1/',
  '/package/endpoint/8.15.2/',
  '/package/endpoint/0.13.1/',
  '/package/apache/1.14.0/',
  '/package/log/2.4.4/',
  '/package/log/2.4.0/',
  '/package/pad/0.6.4/',
  '/package/lmd/2.5.2/',
  '/package/elasticsearch/1.19.0/',
  '/package/kibana/2.8.0/',
  '/package/system/2.7.0/',
  '/package/system/2.6.3/',
  '/package/system/2.6.2/',
  '/package/system/2.5.2/',
  '/package/system/1.62.1/',
  '/package/system/1.38.1/',
  '/package/network_traffic/1.33.0/',
  '/package/security_ai_prompts/1.0.10/',
  '/package/elastic_connectors/1.0.3/',
  '/package/winlog/2.4.0/',
  '/package/cloud_security_posture/3.1.1/',
  '/package/cloud_security_posture/1.12.0/',
  '/package/security_detection_engine/9.2.1/',
  '/package/entityanalytics_ad/0.17.0/',
  '/package/httpjson/1.22.0/',
  '/package/prometheus/1.24.2/',
  '/package/linux/0.6.11/',
  '/package/nginx_ingress_controller_otel/0.1.0/',
  '/package/synthetics/1.3.0/',
  '/package/netflow/2.23.1/',
  '/package/windows/3.1.0/',
]

const statics = [
  '/package/zoom/1.2.1/changelog.yml',
  '/package/aws/1.16.4/changelog.yml',
]

const artifacts = []
// const artifacts = [
//   '/epr/zoom/zoom-1.2.1.zip',
//   '/epr/aws/aws-1.16.4.zip',
// ]

const searches = [
  '/search',
  '/search?all=true&prerelease=true',
  '/search?all=true',
  '/search?spec.min=1.0&spec.max=3.0',
  '/search?spec.min=2.0&spec.max=3.0',
  '/search?spec.min=2.2&spec.max=3.1',
  '/search?spec.min=3.0&spec.max=3.5',
  '/search?prerelease=true&spec.min=2.3&spec.max=3.5',
  '/search?spec.min=2.0&spec.max=3.0&prerelease=true',
  '/search?capabilities=security&spec.min=3.0&spec.max=3.5',
  '/search?prerelease=true&capabilities=security&spec.min=3.0&spec.max=3.5',
  '/search?prerelease=true&capabilities=security',
  '/search?prerelease=true&capabilities=observability',
  '/search?prerelease=true&capabilities=observability,security',
  '/search?prerelease=true&category=security',
  '/search?prerelease=true&category=observability',
  '/search?package=aws',
  '/search?package=security_detection_engine',
  '/search?package=security_detection_engine&prerelease=true',
  '/search?package=elastic_package_registry',
  '/search?package=nginx',
  '/search?package=nginx&prerelease=true',
  '/search?package=system&prerelease=true&spec.min=2.3&spec.max=3.5',
  '/search?package=system&prerelease=true&kibana.version=8.17.0',  
  '/search?package=synthetics&prerelease=true&kibana.version=8.17.10',
  '/search?type=integration',
  '/search?type=content',
  '/search?type=content&kibana.version=9.2.1&spec.min=2.3&spec.max=3.5',
  '/search?package=aws&all=true',
  '/search?kibana.version=9.3.0&spec.min=2.3&spec.max=3.5',
  '/search?kibana.version=9.2.1&spec.min=2.3&spec.max=3.5',
  '/search?kibana.version=9.2.0&spec.min=2.3&spec.max=3.5',
  '/search?prerelease=true&kibana.version=9.2.0&spec.min=2.3&spec.max=3.5',
  '/search?kibana.version=9.1.6&spec.min=2.3&spec.max=3.4',
  '/search?kibana.version=9.1.5&spec.min=2.3&spec.max=3.4',
  '/search?kibana.version=9.1.4&spec.min=2.3&spec.max=3.4',
  '/search?prerelease=true&kibana.version=9.1.5&spec.min=2.3&spec.max=3.4',
  '/search?type=content&capabilities=apm,observability,uptime&spec.min=3.0&spec.max=3.5',
  '/search?kibana.version=9.0.0&prerelease=true',
  '/search?kibana.version=9.3.0&spec.min=2.3&spec.max=3.5',
  '/search?kibana.version=9.0.0',
  '/search?kibana.version=8.19.6',
  '/search?kibana.version=8.19.5',
  '/search?kibana.version=8.19.4',
  '/search?kibana.version=8.19.3',
  '/search?kibana.version=8.19.2',
  '/search?kibana.version=8.18.8',
  '/search?prerelease=true&kibana.version=8.18.8',
  '/search?kibana.version=8.18.5',
  '/search?kibana.version=8.18.4',
  '/search?kibana.version=8.18.2',
  '/search?kibana.version=8.18.1',
  '/search?kibana.version=8.18.0',
  '/search?kibana.version=8.17.4',
  '/search?kibana.version=8.17.3',
  '/search?kibana.version=8.17.1',
  '/search?kibana.version=8.16.0',
  '/search?kibana.version=8.12.1',
  '/search?kibana.version=8.1.0',
  '/search?kibana.version=8.7.0',
  '/search?kibana.version=9.1.0',
  '/search?package=synthetics&experimental=true&kibana.version=8.5.0',
  // add more combinations as needed
];

const categories = [
  '/categories',
  '/categories?prerelease=true',
  '/categories?prerelease=true&include_policy_templates=true',
  '/categories?spec.min=2.0&spec.max=3.0',
  '/categories?spec.min=2.2&spec.max=3.1',
  '/categories?spec.min=2.0&spec.max=3.0&prerelease=true',
  '/categories?kibana.version=9.3.0&spec.min=2.3&spec.max=3.5',
  '/categories?kibana.version=9.2.1&spec.min=2.3&spec.max=3.5',
  '/categories?kibana.version=9.2.0&spec.min=2.3&spec.max=3.5',
  '/categories?kibana.version=9.1.6&spec.min=2.3&spec.max=3.4',
  '/categories?kibana.version=9.1.4&spec.min=2.3&spec.max=3.4',
  '/categories?kibana.version=9.1.2&spec.min=2.3&spec.max=3.4',
  '/categories?kibana.version=9.1.1&spec.min=2.3&spec.max=3.4',
  '/categories?kibana.version=9.0.3&spec.min=2.3&spec.max=3.3',
  '/categories?prerelease=true&kibana.version=9.1.6&spec.min=2.3&spec.max=3.4',
  '/categories?prerelease=true&kibana.version=9.0.3&spec.min=2.3&spec.max=3.3',
  '/categories?prerelease=true&capabilities=security',
  '/categories?prerelease=true&capabilities=observability',
  '/categories?prerelease=true&capabilities=observability,security',
  '/categories?capabilities=security&spec.min=3.0&spec.max=3.5',
  '/categories?capabilities=apm,observability,uptime&spec.min=3.0&spec.max=3.5',
  '/categories?prerelease=true&capabilities=security&spec.min=3.0&spec.max=3.5',
  '/categories?kibana.version=9.2.0',
  '/categories?kibana.version=9.1.0',
  '/categories?kibana.version=9.0.0&prerelease=true',
  '/categories?kibana.version=9.0.0',
  '/categories?kibana.version=8.19.3&prerelease=true',
  '/categories?kibana.version=8.19.6',
  '/categories?kibana.version=8.19.5',
  '/categories?kibana.version=8.19.4',
  '/categories?kibana.version=8.19.3',
  '/categories?kibana.version=8.19.2',
  '/categories?kibana.version=8.19.2&prerelease=true',
  '/categories?kibana.version=8.18.8',
  '/categories?kibana.version=8.18.2',
  '/categories?kibana.version=8.18.1',
  '/categories?kibana.version=8.18.0',
  '/categories?kibana.version=8.17.8',
  '/categories?kibana.version=8.17.8&prerelease=true',
  '/categories?kibana.version=8.17.1',
  '/categories?kibana.version=8.16.0',
  '/categories?kibana.version=8.13.4',
  '/categories?kibana.version=8.1.0',
  '/categories?kibana.version=8.7.0',
  // add more combinations as needed
];

const minSleep = 1;
const maxSleep = 2;

export default function () {
  // group('Core Endpoints', () => {
  //   http.get(`${BASE}/`, { tags: { endpoint: 'root' } });
  //   http.get(`${BASE}/health`, { tags: { endpoint: 'health' } });
  //   http.get(`${BASE}/favicon.ico`, { tags: { endpoint: 'favicon' } });
  // });
  
  // Set an initial sleep to randomize the start time of VUs
  sleep(randomIntBetween(minSleep, maxSleep));

  group('Package & EPR', () => {
    packages.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "package" } });
      check(res, { 'status 200': r => r.status === 200 });
      sleep(randomIntBetween(minSleep, maxSleep))
    });
    artifacts.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "artifacts" } });
      check(res, { 'status 200': r => r.status === 200 });
      sleep(randomIntBetween(minSleep, maxSleep))
    });
  });

  group('Statics', () => {
    statics.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "statics" } });
      check(res, { 'status 200': r => r.status === 200 });
      sleep(randomIntBetween(minSleep, maxSleep))
    });
  });


  group('Search', () => {
    // Test /search with varying query parameters
    searches.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "search" } });
      check(res, { 'status 200': r => r.status === 200 });
      sleep(randomIntBetween(minSleep, maxSleep))
    });
  });

  group('Categories', () => {
    // Test /categories with varying query parameters
    categories.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "categories" } });
      check(res, { 'status 200': r => r.status === 200 });
      sleep(randomIntBetween(minSleep, maxSleep))
    });
  });
}

