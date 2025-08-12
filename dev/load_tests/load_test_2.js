import http from 'k6/http';
import { check, group, sleep } from 'k6';

const BASE = `${__ENV.TARGET_HOST}`;

const packages = [
  '/package/zoom/1.2.1/',
  '/package/aws/1.16.4/',
]

const statics = [
  '/package/zoom/1.2.1/changelog.yml',
  '/package/aws/1.16.4/changelog.yml',
]

const artifacts = [
  '/epr/zoom/zoom-1.2.1.zip',
  '/epr/aws/aws-1.16.4.zip',
]

const searches = [
  '/search',
  '/search?all=true&prerelease=true',
  '/search?all=true&prerelease=false',
  '/search?spec.min=2.0&spec.max=3.0',
  '/search?spec.min=2.0&spec.max=3.0&prerelease=true',
  '/search?prerelease=true&capabilities=security',
  '/search?prerelease=true&categories=security',
  '/search?package=aws',
  '/search?package=security_detection_engine',
  '/search?type=integration',
  '/search?package=aws&all=true',
  '/search?kibana.version=9.0.0',
  // add more combinations as needed
];

const categories = [
  '/categories',
  '/categories?all=true&prerelease=true',
  '/categories?all=true&prerelease=false',
  '/categories?spec.min=2.0&spec.max=3.0',
  '/categories?spec.min=2.0&spec.max=3.0&prerelease=true',
  '/categories?prerelease=true&categories=security',
  '/categories?prerelease=true&capabilities=security',
  '/categories?kibana.version=9.0.0',
  // add more combinations as needed
];

export default function () {
  group('Core Endpoints', () => {
    http.get(`${BASE}/`, { tags: { endpoint: 'root' } });
    http.get(`${BASE}/health`, { tags: { endpoint: 'health' } });
    http.get(`${BASE}/favicon.ico`, { tags: { endpoint: 'favicon' } });
  });

  group('Package & EPR', () => {
    packages.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "package" } });
      check(res, { 'status 200': r => r.status === 200 });
    });
    artifacts.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "artifacts" } });
      check(res, { 'status 200': r => r.status === 200 });
    });
  });

  group('Statics', () => {
    statics.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "statics" } });
      check(res, { 'status 200': r => r.status === 200 });
    });
  });


  group('Search', () => {
    // Test /search with varying query parameters
    searches.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "search" } });
      check(res, { 'status 200': r => r.status === 200 });
    });
  });

  group('Categories', () => {
    // Test /categories with varying query parameters
    categories.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "categories" } });
      check(res, { 'status 200': r => r.status === 200 });
    });
  });


  sleep(1);
}

