package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	lighttaskscheduler "github.com/memory-overflow/light-task-scheduler"
	memeorycontainer "github.com/memory-overflow/light-task-scheduler/container/memory_container"
	videocut "github.com/memory-overflow/light-task-scheduler/example/videocut_example/video_cut"
)

func main() {
	inputVideo := os.Args[1]
	mode := "queue" // 默认队列容器
	if len(os.Args) > 2 {
		mode = os.Args[2] // 任务容器模式
	}

	go videocut.StartServer() // start video cut microservice

	// 构建队列容器，队列长度 10000
	var container lighttaskscheduler.TaskContainer
	var persistencer lighttaskscheduler.TaskdataPersistencer
	var scanInterval time.Duration // 调度器扫描间隔时间

	if mode == "queue" {
		persistencer = nil
		container = memeorycontainer.MakeQueueContainer(10000, 100*time.Millisecond)
		scanInterval = 50 * time.Millisecond
	} else if mode == "sql" {
		var err error
		videocutContainer, err := videocut.MakeVideoCutSqlContainer("127.0.0.1", "3306", "root", "", "test")
		if err != nil {
			log.Fatalf("build container failed: %v\n", err)
		}
		persistencer = videocutContainer
		container = videocutContainer
		scanInterval = 2 * time.Second
	}

	// 构建裁剪任务执行器
	actuator := videocut.MakeVideoCutActuator()
	sch, err := lighttaskscheduler.MakeScheduler(
		container, actuator, persistencer,
		lighttaskscheduler.Config{
			TaskLimit:              2,                // 2 并发
			TaskTimeout:            20 * time.Second, // 20s 超时时间
			EnableFinshedTaskList:  true,             // 开启已完成任务返回功能
			SchedulingPollInterval: scanInterval,
			DisableStatePoll:       false,
			StatePollInterval:      scanInterval,
		},
	)
	if err != nil {
		log.Fatalf("make schedule failed: %v\n", err)
	}
	// 添加任务，把视频裁前 100s 剪成 10s 一个的视频
	for i := 0; i < 100; i += 10 {
		// 这里的任务 ID 是为了调度框架方便标识唯一任务的ID, 和微服务的任务ID不同，是映射关系
		taskId := "task-" + videocut.GenerateRandomString(8)
		if err := sch.AddTask(context.Background(),
			lighttaskscheduler.Task{
				TaskId: taskId,
				TaskItem: videocut.VideoCutTask{
					TaskId:       taskId,
					InputVideo:   inputVideo,
					CutStartTime: float32(i),
					CutEndTime:   float32(i + 10),
				},
			}); err != nil {
			log.Printf("add task TaskId %s failed: %v\n", strconv.Itoa(i), err)
		}
	}

	for task := range sch.FinshedTasks() {
		if task.TaskStatus == lighttaskscheduler.TASK_STATUS_FAILED {
			log.Printf("failed task %s, reason: %s, timecost: %dms\n",
				task.TaskId, task.FailedReason, task.TaskEndTime.Sub(task.TaskStartTime).Milliseconds())
		} else if task.TaskStatus == lighttaskscheduler.TASK_STATUS_SUCCESS {
			log.Printf("success task %s, timecost: %dms\n", task.TaskId,
				task.TaskEndTime.Sub(task.TaskStartTime).Milliseconds())
		}
	}
}
