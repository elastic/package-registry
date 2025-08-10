import http from 'k6/http';
import { check, group, sleep } from 'k6';

const BASE = 'http://localhost:8080';

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
  '',
  '?all=true&prerelease=true',
  '?all=true&prerelease=false',
  '?spec.min=2.0&spec.max=3.0',
  '?spec.min=2.0&spec.max=3.0&prerelease=true',
  '?prerelease=true&capabilities=security',
  '?prerelease=true&categories=security',
  '?package=aws',
  '?type=integration',
  '?package=aws&all=true',
  '?kibana.version=9.0.0',
  // add more combinations as needed
];

const categories = [
  '',
  '?all=true&prerelease=true',
  '?all=true&prerelease=false',
  '?spec.min=2.0&spec.max=3.0',
  '?spec.min=2.0&spec.max=3.0&prerelease=true',
  '?prerelease=true&categories=security',
  '?prerelease=true&capabilities=security',
  '?kibana.version=9.0.0',
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


  group('Search & Categories', () => {
    // Test /search with varying query parameters
    searches.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "search" } });
      check(res, { 'status 200': r => r.status === 200 });
    });
    // Similarly for /categories
    categories.forEach((path) => {
      const res = http.get(`${BASE}${path}`, { tags: { endpoint: "categories" } });
      check(res, { 'status 200': r => r.status === 200 });
    });
  });


  sleep(1);
}

