print('Hello Tiltfile')

load('ext://pack', 'pack')
update_settings( max_parallel_updates = 5, k8s_upsert_timeout_secs = 200 )

pack('controller',
    deps = ['./cmd/controller/build'],
    builder='gcriocfbuildservice/kpack-builder:latest',
    live_update = [ sync('./cmd/controller/build', '/tmp/tilt'),
                    run('cp -rf /tmp/tilt/* /layers/paketo-buildpacks_go-build/targets/bin', trigger=['./cmd/controller/build']),
                   ],
    env_vars=['BP_GO_TARGETS=./cmd/controller', 'BP_LIVE_RELOAD_ENABLED=true']
     )
pack('webhook',
     deps = ['./cmd/webhook/build'],
     builder='gcriocfbuildservice/kpack-builder:latest',
     live_update = [ sync('./cmd/webhook/build', '/tmp/tilt'),
                     run('cp -rf /tmp/tilt/* /layers/paketo-buildpacks_go-build/targets/bin', trigger=['./cmd/webhook/build']),
                    ],
     env_vars=['BP_GO_TARGETS=./cmd/webhook', 'BP_LIVE_RELOAD_ENABLED=true']
     )
pack('build-init',
     deps = ['./cmd/build-init/build'],
     builder='gcriocfbuildservice/kpack-builder:latest',
     live_update = [ sync('./cmd/build-init/build', '/tmp/tilt'),
                     run('cp -rf /tmp/tilt/* /layers/paketo-buildpacks_go-build/targets/bin', trigger=['./cmd/build-init/build']),
                    ],
     env_vars=['BP_GO_TARGETS=./cmd/build-init', 'BP_LIVE_RELOAD_ENABLED=true']
     )
pack('build-waiter',
     deps = ['./cmd/build-waiter/build'],
     builder='gcriocfbuildservice/kpack-builder:latest',
     live_update = [ sync('./cmd/build-waiter/build', '/tmp/tilt'),
                     run('cp -rf /tmp/tilt/* /layers/paketo-buildpacks_go-build/targets/bin', trigger=['./cmd/build-waiter/build']),
                    ],
     env_vars=['BP_GO_TARGETS=./cmd/build-waiter', 'BP_LIVE_RELOAD_ENABLED=true']
     )
pack('rebase',
     deps = ['./cmd/rebase/build'],
     builder='gcriocfbuildservice/kpack-builder:latest',
     live_update = [ sync('./cmd/rebase/build', '/tmp/tilt'),
                     run('cp -rf /tmp/tilt/* /layers/paketo-buildpacks_go-build/targets/bin', trigger=['./cmd/rebase/build']),
                    ],
     env_vars=['BP_GO_TARGETS=./cmd/rebase', 'BP_LIVE_RELOAD_ENABLED=true']
     )
pack('completion',
     deps = ['./cmd/completion/build'],
     builder='gcriocfbuildservice/kpack-builder:latest',
     live_update = [ sync('./cmd/completion/build', '/tmp/tilt'),
                     run('cp -rf /tmp/tilt/* /layers/paketo-buildpacks_go-build/targets/bin/', trigger=['./cmd/completion/build']),
                    ],
     env_vars=['BP_GO_TARGETS=./cmd/completion', 'BP_LIVE_RELOAD_ENABLED=true']
     )


local_resource('controller',
  cmd='GOOS=linux GOARCH=amd64 go build -o build/ -buildmode pie .',
  deps=['./cmd/controller'],
  ignore=['./cmd/controller/build'],
  dir='./cmd/controller'
)

local_resource('webhook',
  cmd='GOOS=linux GOARCH=amd64 go build -o build/ -buildmode pie .',
  deps=['./cmd/webhook'],
  ignore=['./cmd/webhook/build'],
  dir='./cmd/webhook'
)

local_resource('build-init',
  cmd='GOOS=linux GOARCH=amd64 go build -o build/ -buildmode pie .',
  deps=['./cmd/build-init'],
  ignore=['./cmd/build-init/build'],
  dir='./cmd/build-init'
)

local_resource('build-waiter',
  cmd='GOOS=linux GOARCH=amd64 go build -o build/ -buildmode pie .',
  deps=['./cmd/build-waiter'],
  ignore=['./cmd/build-waiter/build'],
  dir='./cmd/build-waiter'
)

local_resource('rebase',
  cmd='GOOS=linux GOARCH=amd64 go build -o build/ -buildmode pie .',
  deps=['./cmd/rebase'],
  ignore=['./cmd/rebase/build'],
  dir='./cmd/rebase'
)

local_resource('completion',
  cmd='GOOS=linux GOARCH=amd64 go build -o build/ -buildmode pie .',
  deps=['./cmd/completion'],
  ignore=['./cmd/completion/build'],
  dir='./cmd/completion'
)

ytt_text = local('./hack/deploytilt.sh')

k8s_yaml(ytt_text)

k8s_kind('ConfigMap', image_json_path='{.data.image}', pod_readiness='ignore')
